package restate

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/restatedev/sdk-go/encoding"
	"google.golang.org/protobuf/proto"
)

type serviceNamer interface {
	ServiceName() string
}

var (
	typeOfContext       = reflect.TypeOf((*Context)(nil)).Elem()
	typeOfObjectContext = reflect.TypeOf((*ObjectContext)(nil)).Elem()
	typeOfVoid          = reflect.TypeOf((*Void)(nil))
	typeOfError         = reflect.TypeOf((*error)(nil))
)

// Object converts a struct with methods into a Virtual Object where each correctly-typed
// and exported method of the struct will become a handler on the Object. The Object name defaults
// to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should be of the type `ObjectHandlerFn[I, O]`.
// Input types I will be deserialised from JSON except when they are restate.Void,
// in which case no input bytes or content type may be sent. Output types O will be serialised
// to JSON except when they are restate.Void, in which case no data will be sent and no content type
// set.
func Object(object any) *ObjectRouter {
	typ := reflect.TypeOf(object)
	val := reflect.ValueOf(object)
	var name string
	if sn, ok := object.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}
	router := NewObjectRouter(name)

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

		if ctxType := mtype.In(1); ctxType != typeOfObjectContext {
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

		router.Handler(mname, &objectReflectHandler{
			reflectHandler{
				fn:       method.Func,
				receiver: val,
				input:    mtype.In(2),
				output:   mtype.Out(0),
			},
		})
	}

	return router
}

// Service converts a struct with methods into a Restate Service where each correctly-typed
// and exported method of the struct will become a handler on the Service. The Service name defaults
// to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should be of the type `ServiceHandlerFn[I, O]`.
// Input types I will be deserialised from JSON except when they are restate.Void,
// in which case no input bytes or content type may be sent. Output types O will be serialised
// to JSON except when they are restate.Void, in which case no data will be sent and no content type
// set.
func Service(service any) *ServiceRouter {
	typ := reflect.TypeOf(service)
	val := reflect.ValueOf(service)
	var name string
	if sn, ok := service.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}
	router := NewServiceRouter(name)

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

		router.Handler(mname, &serviceReflectHandler{
			reflectHandler{
				fn:       method.Func,
				receiver: val,
				input:    mtype.In(2),
				output:   mtype.Out(0),
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

func (h *reflectHandler) InputPayload() *encoding.InputPayload {
	if h.input == typeOfVoid {
		return &encoding.InputPayload{}
	} else {
		return &encoding.InputPayload{
			Required:    true,
			ContentType: proto.String("application/json"),
		}
	}
}

func (h *reflectHandler) OutputPayload() *encoding.OutputPayload {
	if h.output == typeOfVoid {
		return &encoding.OutputPayload{}
	} else {
		return &encoding.OutputPayload{
			ContentType: proto.String("application/json"),
		}
	}
}

func (h *reflectHandler) sealed() {}

type objectReflectHandler struct {
	reflectHandler
}

var _ Handler = (*objectReflectHandler)(nil)

func (h *objectReflectHandler) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if h.input != typeOfVoid {
		if err := json.Unmarshal(bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err))
		}
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

	if h.output == typeOfVoid {
		return nil, nil
	}

	bytes, err := json.Marshal(outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

type serviceReflectHandler struct {
	reflectHandler
}

var _ Handler = (*serviceReflectHandler)(nil)

func (h *serviceReflectHandler) Call(ctx Context, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if err := json.Unmarshal(bytes, input.Interface()); err != nil {
		return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
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

	bytes, err := json.Marshal(outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}
