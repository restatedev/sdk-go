package restate

import (
	"encoding/json"
	"fmt"
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

type serviceHandler[I any, O any] struct {
	fn ServiceHandlerFn[I, O]
}

// NewServiceHandler create a new handler for a service
func NewServiceHandler[I any, O any](fn ServiceHandlerFn[I, O]) *serviceHandler[I, O] {
	return &serviceHandler[I, O]{
		fn: fn,
	}
}

func (h *serviceHandler[I, O]) Call(ctx Context, bytes []byte) ([]byte, error) {
	input := new(I)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
		}
	}

	// we are sure about the fn signature so it's safe to do this
	output, err := h.fn(
		ctx,
		*input,
	)
	if err != nil {
		return nil, err
	}

	bytes, err = json.Marshal(output)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *serviceHandler[I, O]) sealed() {}

type objectHandler[I any, O any] struct {
	fn ObjectHandlerFn[I, O]
}

func NewObjectHandler[I any, O any](fn ObjectHandlerFn[I, O]) *objectHandler[I, O] {
	return &objectHandler[I, O]{
		fn: fn,
	}
}

func (h *objectHandler[I, O]) Call(ctx ObjectContext, bytes []byte) ([]byte, error) {
	input := new(I)

	if len(bytes) > 0 {
		// use the zero value if there is no input data at all
		if err := json.Unmarshal(bytes, input); err != nil {
			return nil, TerminalError(fmt.Errorf("request doesn't match handler signature: %w", err))
		}
	}

	output, err := h.fn(
		ctx,
		*input,
	)
	if err != nil {
		return nil, err
	}

	bytes, err = json.Marshal(output)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to serialize output: %w", err))
	}

	return bytes, nil
}

func (h *objectHandler[I, O]) sealed() {}
