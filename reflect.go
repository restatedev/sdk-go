package restate

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/state"
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
// The handler name is the name of the method. Handler methods should have one of the following signatures:
// - (ctx, I) (O, error)
// - (ctx, I) (O)
// - (ctx, I) (error)
// - (ctx, I)
// - (ctx)
// - (ctx) (error)
// - (ctx) (O)
// - (ctx) (O, error)
// Where ctx is [ObjectContext], [ObjectSharedContext] or [Context]. Other signatures are ignored.
// This function will panic if a mixture of object and service method signatures or opts are provided.
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
		// Method needs 2-3 ins: receiver, Context, optionally I
		numIn := mtype.NumIn()
		if numIn < 2 || numIn > 3 {
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

		// Method needs 0-2 outs: (), (O), (error), (O, error) are all valid
		var output reflect.Type
		var hasError bool
		switch mtype.NumOut() {
		case 0:
			// ()
			output = nil
			hasError = false
		case 1:
			if returnType := mtype.Out(0); returnType == typeOfError {
				// (error)
				output = nil
				hasError = true
			} else {
				output = returnType
				hasError = false
			}
		case 2:
			if returnType := mtype.Out(1); returnType != typeOfError {
				continue
			}
			output = mtype.Out(0)
			hasError = true
		default:
			continue
		}

		var input reflect.Type
		if numIn > 2 {
			input = mtype.In(2)
		}

		switch def := definition.(type) {
		case *service:
			def.Handler(mname, &reflectHandler{
				fn:          method.Func,
				receiver:    val,
				input:       input,
				output:      output,
				hasError:    hasError,
				options:     options.HandlerOptions{},
				handlerType: nil,
			},
			)
		case *object:
			def.Handler(mname, &reflectHandler{
				fn:          method.Func,
				receiver:    val,
				input:       input,
				output:      input,
				hasError:    hasError,
				options:     options.HandlerOptions{},
				handlerType: &handlerType,
			},
			)
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
	hasError    bool
	options     options.HandlerOptions
	handlerType *internal.ServiceHandlerType
}

func (h *reflectHandler) GetOptions() *options.HandlerOptions {
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

func (h *reflectHandler) Call(ctx *state.Context, bytes []byte) ([]byte, error) {
	var args []reflect.Value
	if h.input != nil {
		input := reflect.New(h.input)

		if err := encoding.Unmarshal(h.options.Codec, bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
		}

		args = []reflect.Value{h.receiver, reflect.ValueOf(ctxWrapper{ctx}), input.Elem()}
	} else {
		args = []reflect.Value{h.receiver, reflect.ValueOf(ctxWrapper{ctx})}
	}

	output := h.fn.Call(args)
	var outI any

	switch [2]bool{h.output != nil, h.hasError} {
	case [2]bool{false, false}:
		// ()
		return nil, nil
	case [2]bool{false, true}:
		// (error)
		errI := output[0].Interface()
		if errI != nil {
			return nil, errI.(error)
		}
		return nil, nil
	case [2]bool{true, false}:
		// (O)
		outI = output[0].Interface()
	case [2]bool{true, true}:
		// (O, error)
		errI := output[1].Interface()
		if errI != nil {
			return nil, errI.(error)
		}
		outI = output[0].Interface()
	}

	bytes, err := encoding.Marshal(h.options.Codec, outI)
	if err != nil {
		// we don't use a terminal error here as this is hot-fixable by changing the return type
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	return bytes, nil
}

var _ state.Handler = (*reflectHandler)(nil)
