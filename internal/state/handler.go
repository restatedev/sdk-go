package state

import (
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
)

// Handler is implemented by all Restate handlers
type Handler interface {
	GetOptions() *options.HandlerOptions
	InputPayload() *encoding.InputPayload
	OutputPayload() *encoding.OutputPayload
	HandlerType() *internal.ServiceHandlerType
	Call(ctx Context, request []byte) (output []byte, err error)
}
