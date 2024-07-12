package restate

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type serviceNamer interface {
	Name() string
}

var (
	typeOfContext       = reflect.TypeFor[Context]()
	typeOfObjectContext = reflect.TypeFor[ObjectContext]()
	typeOfError         = reflect.TypeFor[error]()
)

func Object(object any) *ObjectRouter {
	typ := reflect.TypeOf(object)
	val := reflect.ValueOf(object)
	var name string
	if sn, ok := object.(serviceNamer); ok {
		name = sn.Name()
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
			fn:       method.Func,
			receiver: val,
			input:    mtype.In(2),
			output:   mtype.Out(0),
		})
	}

	return router
}

func Service(service any) *ServiceRouter {
	typ := reflect.TypeOf(service)
	val := reflect.ValueOf(service)
	var name string
	if sn, ok := service.(serviceNamer); ok {
		name = sn.Name()
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
			fn:       method.Func,
			receiver: val,
			input:    mtype.In(2),
			output:   mtype.Out(0),
		})
	}

	return router
}

type objectReflectHandler struct {
	fn       reflect.Value
	receiver reflect.Value
	input    reflect.Type
	output   reflect.Type
}

func (h *objectReflectHandler) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
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

	bytes, err := json.Marshal(outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *objectReflectHandler) sealed() {}

type serviceReflectHandler struct {
	fn       reflect.Value
	receiver reflect.Value
	input    reflect.Type
	output   reflect.Type
}

func (h *serviceReflectHandler) Call(ctx Context, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
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

	bytes, err := json.Marshal(outI)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *serviceReflectHandler) sealed() {}
