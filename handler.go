package restate

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Void is a placeholder used usually for functions that their signature require that
// you accept an input or return an output but the function implementation does not
// require them
type Void struct{}

func (v Void) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func (v *Void) UnmarshalJSON(_ []byte) error {
	return nil
}

type ServiceHandler struct {
	fn     reflect.Value
	input  reflect.Type
	output reflect.Type
}

// NewServiceHandler create a new handler for a service
func NewServiceHandler[I any, O any](fn ServiceHandlerFn[I, O]) *ServiceHandler {
	return &ServiceHandler{
		fn:     reflect.ValueOf(fn),
		input:  reflect.TypeFor[I](),
		output: reflect.TypeFor[O](),
	}
}

func (h *ServiceHandler) Call(ctx Context, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
		}
	}

	// we are sure about the fn signature so it's safe to do this
	output := h.fn.Call([]reflect.Value{
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

func (h *ServiceHandler) sealed() {}

type ObjectHandler struct {
	fn     reflect.Value
	input  reflect.Type
	output reflect.Type
}

func NewObjectHandler[I any, O any](fn ObjectHandlerFn[I, O]) *ObjectHandler {
	return &ObjectHandler{
		fn:     reflect.ValueOf(fn),
		input:  reflect.TypeFor[I](),
		output: reflect.TypeFor[O](),
	}
}

func (h *ObjectHandler) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := reflect.New(h.input)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
		}
	}

	// we are sure about the fn signature so it's safe to do this
	output := h.fn.Call([]reflect.Value{
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

func (h *ObjectHandler) sealed() {}
