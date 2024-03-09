package router

import (
	"context"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
)

type Context interface {
	Ctx() context.Context
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
}

// UnKeyedHandlerFn signature of `un-keyed` handler function
type UnKeyedHandlerFn[I any, O any] func(ctx Context, input I) (output O, err error)

// KeyedHandlerFn signature for `keyed` handler function
type KeyedHandlerFn[I any, O any] func(ctx Context, key string, input I) (output O, err error)

// Handler interface.
type Handler interface {
	Call(ctx Context, request *dynrpc.RpcRequest) (output *dynrpc.RpcResponse, err error)
	sealed()
}

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
