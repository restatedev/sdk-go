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

// Reflect converts a struct with methods into a service definition where each correctly-typed
// and exported method of the struct will become a handler in the definition. The service name
// defaults to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should be of the type `ServiceHandlerFn[I,O]`,
// `ObjectHandlerFn[I, O]` or `ObjectSharedHandlerFn[I, O]`. This function will panic if a mixture of
// object and service method signatures or opts are provided.
//
// Input types will be deserialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no input bytes or content type may be sent.
// Output types will be serialised with the provided codec (defaults to JSON) except when they are restate.Void,
// in which case no data will be sent and no content type set.
func Reflect(rcvr any, opts ...options.ServiceDefinitionOption) ServiceDefinition {
	typ := reflect.TypeOf(rcvr)
	val := reflect.ValueOf(rcvr)
	var name string
	if sn, ok := rcvr.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}

	var definition ServiceDefinition

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

		var handlerType internal.ServiceHandlerType

		switch mtype.In(1) {
		case typeOfContext:
			if definition == nil {
				definition = NewService(name, opts...)
			} else if definition.Type() != internal.ServiceType_SERVICE {
				panic("found a mix of service context arguments and other context arguments")
			}
		case typeOfObjectContext:
			if definition == nil {
				definition = NewObject(name, opts...)
			} else if definition.Type() != internal.ServiceType_VIRTUAL_OBJECT {
				panic("found a mix of object context arguments and other context arguments")
			}
			handlerType = internal.ServiceHandlerType_EXCLUSIVE
		case typeOfSharedObjectContext:
			if definition == nil {
				definition = NewObject(name, opts...)
			} else if definition.Type() != internal.ServiceType_VIRTUAL_OBJECT {
				panic("found a mix of object context arguments and other context arguments")
			}
			handlerType = internal.ServiceHandlerType_SHARED
		default:
			// first parameter is not a context
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

		switch def := definition.(type) {
		case *service:
			def.Handler(mname, &serviceReflectHandler{
				reflectHandler{
					fn:          method.Func,
					receiver:    val,
					input:       input,
					output:      output,
					options:     options.HandlerOptions{},
					handlerType: nil,
				},
			})
		case *object:
			def.Handler(mname, &objectReflectHandler{
				reflectHandler{
					fn:          method.Func,
					receiver:    val,
					input:       input,
					output:      input,
					options:     options.HandlerOptions{},
					handlerType: &handlerType,
				},
			})
		}
	}

	if definition == nil {
		panic("no valid handlers could be found within the exported methods on this struct")
	}

	return definition
}

type reflectHandler struct {
	fn          reflect.Value
	receiver    reflect.Value
	input       reflect.Type
	output      reflect.Type
	options     options.HandlerOptions
	handlerType *internal.ServiceHandlerType
}

func (h *reflectHandler) getOptions() *options.HandlerOptions {
	return &h.options
}

func (h *reflectHandler) InputPayload() *encoding.InputPayload {
	return encoding.InputPayloadFor(h.options.Codec, reflect.Zero(h.input).Interface())
}

func (h *reflectHandler) OutputPayload() *encoding.OutputPayload {
	return encoding.OutputPayloadFor(h.options.Codec, reflect.Zero(h.output).Interface())
}

func (h *reflectHandler) HandlerType() *internal.ServiceHandlerType {
	return h.handlerType
}

type objectReflectHandler struct {
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
		// we don't use a terminal error here as this is hot-fixable by changing the return type
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	return bytes, nil
}

type serviceReflectHandler struct {
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
		// we don't use a terminal error here as this is hot-fixable by changing the return type
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	return bytes, nil
}
