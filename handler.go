package restate

import (
	"context"
	"reflect"

	"github.com/muhamadazmy/restate-sdk-go/generated/dynrpc"
	"github.com/rs/zerolog/log"
)

type Context interface {
	Ctx() context.Context
}

// UnKeyedHandlerFn signature of `un-keyed` handler function
type UnKeyedHandlerFn[I any, O any] func(ctx Context, input I) (output O, err error)

// KeyedHandlerFn signature for `keyed` handler function
type KeyedHandlerFn[I any, O any] func(ctx Context, key string, input I) (output O, err error)

// Handler interface.
type Handler interface {
	Call(ctx Context, request dynrpc.RpcRequest) (output dynrpc.RpcResponse)
	sealed()
}

type UnKeyedHandler struct {
	fn     reflect.Value
	input  reflect.Type
	output reflect.Type
}

// NewUnKeyedHandler create a new handler for an `un-keyed` function
func NewUnKeyedHandler[I any, O any](fn UnKeyedHandlerFn[I, O]) *UnKeyedHandler {
	return &UnKeyedHandler{
		fn:     reflect.ValueOf(fn),
		input:  reflect.TypeFor[I](),
		output: reflect.TypeFor[O](),
	}
}

func (h *UnKeyedHandler) Call(ctx Context, request dynrpc.RpcRequest) dynrpc.RpcResponse {
	// this is unkeyed, so there is no need for the `key` attribute.
	// we are also sure of the input and output types.
	// input := reflect.New(h.input)
	log.Debug().Msg("call to [UNKIED] function")
	return dynrpc.RpcResponse{}
}

func (h *UnKeyedHandler) sealed() {}

type KeyedHandler struct {
	fn     reflect.Value
	input  reflect.Type
	output reflect.Type
}

func NewKeyedHandler[I any, O any](fn KeyedHandlerFn[I, O]) *KeyedHandler {
	return &KeyedHandler{
		fn:     reflect.ValueOf(fn),
		input:  reflect.TypeFor[I](),
		output: reflect.TypeFor[O](),
	}
}

func (h *KeyedHandler) Call(ctx Context, request dynrpc.RpcRequest) dynrpc.RpcResponse {
	// this is unkeyed, so there is no need for the `key` attribute.
	// we are also sure of the input and output types.
	// input := reflect.New(h.input)
	log.Debug().Msg("call to [KEYED] function")
	return dynrpc.RpcResponse{}
}

func (h *KeyedHandler) sealed() {}

type Router interface {
	Keyed() bool
	Handlers() map[string]Handler
}

type UnKeyedRouter struct {
	handlers map[string]Handler
}

func NewUnKeyedRouter() *UnKeyedRouter {
	return &UnKeyedRouter{
		handlers: make(map[string]Handler),
	}
}

func (r *UnKeyedRouter) Handler(name string, handler *UnKeyedHandler) *UnKeyedRouter {
	r.handlers[name] = handler
	return r
}

func (r *UnKeyedRouter) Keyed() bool {
	return false
}

func (r *UnKeyedRouter) Handlers() map[string]Handler {
	return r.handlers
}

type KeyedRouter struct {
	handlers map[string]Handler
}

func NewKeyedRouter() *KeyedRouter {
	return &KeyedRouter{
		handlers: make(map[string]Handler),
	}
}

func (r *KeyedRouter) Handler(name string, handler *KeyedHandler) *KeyedRouter {
	r.handlers[name] = handler
	return r
}

func (r *KeyedRouter) Keyed() bool {
	return true
}

func (r *KeyedRouter) Handlers() map[string]Handler {
	return r.handlers
}
