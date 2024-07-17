package restate

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
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
// and exported method of the struct will become a handler on the Object. The Object name
// defaults to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should be of the type `ObjectHandlerFn[I, O]` or `ObjectSharedHandlerFn[I, O]`.
//
// Input types will be deserialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no input bytes or content type may be sent.
// Output types will be serialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no data will be sent and no content type set.
func Object(object any, opts ...options.ObjectRouterOption) *ObjectRouter {
	typ := reflect.TypeOf(object)
	val := reflect.ValueOf(object)
	var name string
	if sn, ok := object.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}
	router := NewObjectRouter(name, opts...)

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

		router.Handler(mname, &objectReflectHandler{
			options.ObjectHandlerOptions{},
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
//
// Input types will be deserialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no input bytes or content type may be sent.
// Output types will be serialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no data will be sent and no content type set.
func Service(service any, opts ...options.ServiceRouterOption) *ServiceRouter {
	typ := reflect.TypeOf(service)
	val := reflect.ValueOf(service)
	var name string
	if sn, ok := service.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}
	router := NewServiceRouter(name, opts...)

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

		router.Handler(mname, &serviceReflectHandler{
			options.ServiceHandlerOptions{},
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
	options     options.ObjectHandlerOptions
	handlerType internal.ServiceHandlerType
	reflectHandler
}

var _ ObjectHandler = (*objectReflectHandler)(nil)

func (h *objectReflectHandler) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if err := encoding.Unmarshal(h.options.Codec, bytes, input.Interface()); err != nil {
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

	bytes, err := encoding.Marshal(h.options.Codec, outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *objectReflectHandler) getOptions() *options.ObjectHandlerOptions {
	return &h.options
}

func (h *objectReflectHandler) InputPayload() *encoding.InputPayload {
	return encoding.InputPayloadFor(h.options.Codec, reflect.Zero(h.input).Interface())
}

func (h *objectReflectHandler) OutputPayload() *encoding.OutputPayload {
	return encoding.OutputPayloadFor(h.options.Codec, reflect.Zero(h.output).Interface())
}

func (h *objectReflectHandler) HandlerType() *internal.ServiceHandlerType {
	return &h.handlerType
}

type serviceReflectHandler struct {
	options options.ServiceHandlerOptions
	reflectHandler
}

var _ ServiceHandler = (*serviceReflectHandler)(nil)

func (h *serviceReflectHandler) Call(ctx Context, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if err := encoding.Unmarshal(h.options.Codec, bytes, input.Interface()); err != nil {
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

	bytes, err := encoding.Marshal(h.options.Codec, outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *serviceReflectHandler) getOptions() *options.ServiceHandlerOptions {
	return &h.options
}

func (h *serviceReflectHandler) InputPayload() *encoding.InputPayload {
	return h.options.Codec.InputPayload(reflect.Zero(h.input))
}

func (h *serviceReflectHandler) OutputPayload() *encoding.OutputPayload {
	return h.options.Codec.OutputPayload(reflect.Zero(h.output))
}

func (h *serviceReflectHandler) HandlerType() *internal.ServiceHandlerType {
	return nil
}
