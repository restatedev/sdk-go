package restate

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/muhamadazmy/restate-sdk-go/generated/proto/dynrpc"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	//DefaultBackoffPolicy is an infinite exponential backoff
	DefaultBackoffPolicy = backoff.ExponentialBackOff{
		InitialInterval:     10 * time.Microsecond,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         backoff.DefaultMaxInterval,
		MaxElapsedTime:      0,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
)

type Call interface {
	// Do makes a call and wait for the response
	Do(key string, input any, output any) error
	// Send makes a call in the background (doesn't wait for response) after delay duration
	Send(key string, body any, delay time.Duration) error
}

type Service interface {
	// Method creates a call to method with name
	Method(method string) Call
}

type Context interface {
	Ctx() context.Context
	// Set sets key value to bytes array. You can
	// Note: Use SetAs helper function to seamlessly store
	// a value of specific type.
	Set(key string, value []byte) error
	// Get gets value (bytes array) associated with key
	// If key does not exist, this function return a nil bytes array
	// and a nil error
	// Note: Use GetAs helper function to seamlessly get value
	// as specific type.
	Get(key string) ([]byte, error)
	// Clear deletes a key
	Clear(key string) error
	// ClearAll drops all stored state associated with key
	ClearAll() error
	// Keys returns a list of all associated key
	Keys() ([]string, error)
	// Sleep sleep during the execution until time is reached
	Sleep(until time.Time) error
	// Service gets a Service accessor by name where service
	// must be another service known by restate runtime
	Service(service string) Service

	// SideEffects runs the function (fn) with backoff strategy bo until it succeeds
	// or permanently fail.
	// this stores the results of the function inside restate runtime so a replay
	// will produce the same value (think generating a unique id for example)
	// Note: use the SideEffectAs helper function
	SideEffect(fn func() ([]byte, error), bo ...backoff.BackOff) ([]byte, error)
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

// GetAs helper function to get a key as specific type. Note that
// if there is no associated value with key, an error ErrKeyNotFound is
// returned
// it does encoding/decoding of bytes automatically using msgpack
func GetAs[T any](ctx Context, key string) (output T, err error) {

	bytes, err := ctx.Get(key)
	if err != nil {
		return output, err
	}

	if bytes == nil {
		// key does not exit.
		return output, ErrKeyNotFound
	}

	err = msgpack.Unmarshal(bytes, &output)

	return
}

// SetAs helper function to set a key value with a generic type T.
// it does encoding/decoding of bytes automatically using msgpack
func SetAs[T any](ctx Context, key string, value T) error {
	bytes, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}

	return ctx.Set(key, bytes)
}

// SideEffectAs helper function runs a side effect function with specific concrete type as a result
// it does encoding/decoding of bytes automatically using msgpack
func SideEffectAs[T any](ctx Context, fn func() (T, error), bo ...backoff.BackOff) (output T, err error) {
	bytes, err := ctx.SideEffect(func() ([]byte, error) {
		out, err := fn()
		if err != nil {
			return nil, err
		}

		bytes, err := msgpack.Marshal(out)
		return bytes, TerminalError(err)
	}, bo...)

	if err != nil {
		return output, err
	}

	err = msgpack.Unmarshal(bytes, &output)

	return output, TerminalError(err)
}
