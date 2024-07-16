package restate

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
)

type serviceNamer interface {
	ServiceName() string
}

var (
	typeOfContext             = reflect.TypeOf((*Context)(nil)).Elem()
	typeOfObjectContext       = reflect.TypeOf((*ObjectContext)(nil)).Elem()
	typeOfSharedObjectContext = reflect.TypeOf((*ObjectSharedContext)(nil)).Elem()
	typeOfVoid                = reflect.TypeOf((*Void)(nil)).Elem()
	typeOfError               = reflect.TypeOf((*error)(nil)).Elem()
)

// Object converts a struct with methods into a Virtual Object where each correctly-typed
// and exported method of the struct will become a handler on the Object. The Object name defaults
// to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should be of the type `ObjectHandlerFn[I, O]` or `ObjectSharedHandlerFn[I, O]`.
// Input types I will be deserialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no input bytes or content type may be sent.
// Output types O will be serialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no data will be sent and no content type set.
func Object(object any, options ...ObjectRouterOption) *ObjectRouter {
	typ := reflect.TypeOf(object)
	val := reflect.ValueOf(object)
	var name string
	if sn, ok := object.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}
	router := NewObjectRouter(name, options...)

	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if !method.IsExported() {
			continue
		}
		// Method needs three ins: receiver, ObjectContext, I
		if mtype.NumIn() != 3 {
			continue
		}

		var handlerType internal.ServiceHandlerType

		switch mtype.In(1) {
		case typeOfObjectContext:
			handlerType = internal.ServiceHandlerType_EXCLUSIVE
		case typeOfSharedObjectContext:
			handlerType = internal.ServiceHandlerType_SHARED
		default:
			// first parameter is not an object context
			continue
		}

		// Method needs two outs: O, and error
		if mtype.NumOut() != 2 {
			continue
		}

		// The second return type of the method must be error.
		if returnType := mtype.Out(1); returnType != typeOfError {
			continue
		}

		input := mtype.In(2)
		output := mtype.Out(0)

		var codec encoding.PayloadCodec
		switch {
		case input == typeOfVoid && output == typeOfVoid:
			codec = encoding.VoidCodec
		case input == typeOfVoid:
			codec = encoding.PairCodec{Input: encoding.VoidCodec, Output: nil}
		case output == typeOfVoid:
			codec = encoding.PairCodec{Input: nil, Output: encoding.VoidCodec}
		default:
			codec = nil
		}

		router.Handler(mname, &objectReflectHandler{
			objectHandlerOptions{codec},
			handlerType,
			reflectHandler{
				fn:       method.Func,
				receiver: val,
				input:    input,
				output:   output,
			},
		})
	}

	return router
}

// Service converts a struct with methods into a Restate Service where each correctly-typed
// and exported method of the struct will become a handler on the Service. The Service name defaults
// to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should be of the type `ServiceHandlerFn[I, O]`.
// Input types I will be deserialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no input bytes or content type may be sent.
// Output types O will be serialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no data will be sent and no content type set.
func Service(service any, options ...ServiceRouterOption) *ServiceRouter {
	typ := reflect.TypeOf(service)
	val := reflect.ValueOf(service)
	var name string
	if sn, ok := service.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}
	router := NewServiceRouter(name, options...)

	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)

		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if !method.IsExported() {
			continue
		}

		// Method needs three ins: receiver, Context, I
		if mtype.NumIn() != 3 {
			continue
		}

		if ctxType := mtype.In(1); ctxType != typeOfContext {
			continue
		}

		// Method needs two outs: O, and error
		if mtype.NumOut() != 2 {
			continue
		}

		// The second return type of the method must be error.
		if returnType := mtype.Out(1); returnType != typeOfError {
			continue
		}

		input := mtype.In(2)
		output := mtype.Out(0)

		var codec encoding.PayloadCodec
		switch {
		case input == typeOfVoid && output == typeOfVoid:
			codec = encoding.VoidCodec
		case input == typeOfVoid:
			codec = encoding.PairCodec{Input: encoding.VoidCodec, Output: nil}
		case output == typeOfVoid:
			codec = encoding.PairCodec{Input: nil, Output: encoding.VoidCodec}
		default:
			codec = nil
		}

		router.Handler(mname, &serviceReflectHandler{
			serviceHandlerOptions{codec: codec},
			reflectHandler{
				fn:       method.Func,
				receiver: val,
				input:    input,
				output:   output,
			},
		})
	}

	return router
}

type reflectHandler struct {
	fn       reflect.Value
	receiver reflect.Value
	input    reflect.Type
	output   reflect.Type
}

func (h *reflectHandler) sealed() {}

type objectReflectHandler struct {
	options     objectHandlerOptions
	handlerType internal.ServiceHandlerType
	reflectHandler
}

var _ ObjectHandler = (*objectReflectHandler)(nil)

func (h *objectReflectHandler) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if err := h.options.codec.Unmarshal(bytes, input.Interface()); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	// we are sure about the fn signature so it's safe to do this
	output := h.fn.Call([]reflect.Value{
		h.receiver,
		reflect.ValueOf(ctx),
		input.Elem(),
	})

	outI := output[0].Interface()
	errI := output[1].Interface()
	if errI != nil {
		return nil, errI.(error)
	}

	bytes, err := h.options.codec.Marshal(outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *objectReflectHandler) getOptions() *objectHandlerOptions {
	return &h.options
}

func (h *objectReflectHandler) InputPayload() *encoding.InputPayload {
	return h.options.codec.InputPayload()
}

func (h *objectReflectHandler) OutputPayload() *encoding.OutputPayload {
	return h.options.codec.OutputPayload()
}

func (h *objectReflectHandler) HandlerType() *internal.ServiceHandlerType {
	return &h.handlerType
}

type serviceReflectHandler struct {
	options serviceHandlerOptions
	reflectHandler
}

var _ ServiceHandler = (*serviceReflectHandler)(nil)

func (h *serviceReflectHandler) Call(ctx Context, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if err := h.options.codec.Unmarshal(bytes, input.Interface()); err != nil {
		return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
	}

	// we are sure about the fn signature so it's safe to do this
	output := h.fn.Call([]reflect.Value{
		h.receiver,
		reflect.ValueOf(ctx),
		input.Elem(),
	})

	outI := output[0].Interface()
	errI := output[1].Interface()
	if errI != nil {
		return nil, errI.(error)
	}

	bytes, err := h.options.codec.Marshal(outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *serviceReflectHandler) getOptions() *serviceHandlerOptions {
	return &h.options
}

func (h *serviceReflectHandler) InputPayload() *encoding.InputPayload {
	return h.options.codec.InputPayload()
}

func (h *serviceReflectHandler) OutputPayload() *encoding.OutputPayload {
	return h.options.codec.OutputPayload()
}

func (h *serviceReflectHandler) HandlerType() *internal.ServiceHandlerType {
	return nil
}
