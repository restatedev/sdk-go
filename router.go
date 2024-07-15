package restate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	ErrKeyNotFound = fmt.Errorf("key not found")
)

type CallClient interface {
	// Request makes a call and returns a handle on a future response
	Request(input any) ResponseFuture
}

type SendClient interface {
	// Send makes a call in the background (doesn't wait for response)
	Request(input any) error
}

type ResponseFuture interface {
	// Response waits for the response to the call and unmarshals it into output
	Response(output any) error
	futures.Selectable
}

type ServiceClient interface {
	// Method creates a call to method with name
	Method(method string) CallClient
}

type ServiceSendClient interface {
	// Method creates a call to method with name
	Method(method string) SendClient
}

type Selector interface {
	Remaining() bool
	Select() futures.Selectable
}

type Context interface {
	RunContext

	// Sleep for the duration d
	Sleep(d time.Duration)
	// Return a handle on a sleep duration which can be combined
	After(d time.Duration) After

	// Service gets a Service accessor by name where service
	// must be another service known by restate runtime
	Service(service string) ServiceClient
	// Service gets a Service send accessor by name where service
	// must be another service known by restate runtime
	// and delay is the duration with which to delay requests
	ServiceSend(service string, delay time.Duration) ServiceSendClient

	// Object gets a Object accessor by name where object
	// must be another object known by restate runtime and
	// key is any string representing the key for the object
	Object(object, key string) ServiceClient
	// Object gets a Object accessor by name where object
	// must be another object known by restate runtime,
	// key is any string representing the key for the object,
	// and delay is the duration with which to delay requests
	ObjectSend(object, key string, delay time.Duration) ServiceSendClient

	// Run runs the function (fn) until it succeeds or permanently fails.
	// this stores the results of the function inside restate runtime so a replay
	// will produce the same value (think generating a unique id for example)
	// Note: use the RunAs helper function
	Run(fn func(RunContext) ([]byte, error)) ([]byte, error)

	Awakeable() Awakeable[[]byte]
	ResolveAwakeable(id string, value []byte)
	RejectAwakeable(id string, reason error)

	Selector(futs ...futures.Selectable) (Selector, error)
}

// RunContext methods are the only methods safe to call from inside a .Run()
type RunContext interface {
	context.Context

	// Log obtains a handle on a slog.Logger which already has some useful fields (invocationID and method)
	// By default, this logger will not output messages if the invocation is currently replaying
	// The log handler can be set with `.WithLogger()` on the server object
	Log() *slog.Logger
}

// Router interface
type Router interface {
	Name() string
	Type() internal.ServiceType
	// Set of handlers associated with this router
	Handlers() map[string]Handler
}

type ObjectHandler interface {
	Call(ctx ObjectContext, request []byte) (output []byte, err error)
	Handler
}

type ServiceHandler interface {
	Call(ctx Context, request []byte) (output []byte, err error)
	Handler
}

type Handler interface {
	sealed()
}

type ServiceType string

const (
	ServiceType_VIRTUAL_OBJECT ServiceType = "VIRTUAL_OBJECT"
	ServiceType_SERVICE        ServiceType = "SERVICE"
)

type KeyValueStore interface {
	// Set sets key value to bytes array. You can
	// Note: Use SetAs helper function to seamlessly store
	// a value of specific type.
	Set(key string, value []byte)
	// Get gets value (bytes array) associated with key
	// If key does not exist, this function return a nil bytes array
	// and a nil error
	// Note: Use GetAs helper function to seamlessly get value
	// as specific type.
	Get(key string) ([]byte, error)
	// Clear deletes a key
	Clear(key string)
	// ClearAll drops all stored state associated with key
	ClearAll()
	// Keys returns a list of all associated key
	Keys() ([]string, error)
}

type ObjectContext interface {
	Context
	KeyValueStore
	Key() string
}

// ServiceHandlerFn signature of service (unkeyed) handler function
type ServiceHandlerFn[I any, O any] func(ctx Context, input I) (output O, err error)

// ObjectHandlerFn signature for object (keyed) handler function
type ObjectHandlerFn[I any, O any] func(ctx ObjectContext, input I) (output O, err error)

// ServiceRouter implements Router
type ServiceRouter struct {
	name     string
	handlers map[string]Handler
}

var _ Router = &ServiceRouter{}

// NewServiceRouter creates a new ServiceRouter
func NewServiceRouter(name string) *ServiceRouter {
	return &ServiceRouter{
		name:     name,
		handlers: make(map[string]Handler),
	}
}

func (r *ServiceRouter) Name() string {
	return r.name
}

// Handler registers a new handler by name
func (r *ServiceRouter) Handler(name string, handler ServiceHandler) *ServiceRouter {
	r.handlers[name] = handler
	return r
}

func (r *ServiceRouter) Handlers() map[string]Handler {
	return r.handlers
}

func (r *ServiceRouter) Type() internal.ServiceType {
	return internal.ServiceType_SERVICE
}

// ObjectRouter
type ObjectRouter struct {
	name     string
	handlers map[string]Handler
}

var _ Router = &ObjectRouter{}

func NewObjectRouter(name string) *ObjectRouter {
	return &ObjectRouter{
		name:     name,
		handlers: make(map[string]Handler),
	}
}

func (r *ObjectRouter) Name() string {
	return r.name
}

func (r *ObjectRouter) Handler(name string, handler ObjectHandler) *ObjectRouter {
	r.handlers[name] = handler
	return r
}

func (r *ObjectRouter) Handlers() map[string]Handler {
	return r.handlers
}

func (r *ObjectRouter) Type() internal.ServiceType {
	return internal.ServiceType_VIRTUAL_OBJECT
}

// GetAs helper function to get a key as specific type. Note that
// if there is no associated value with key, an error ErrKeyNotFound is
// returned
// it does encoding/decoding of bytes automatically using msgpack
func GetAs[T any](ctx ObjectContext, key string) (output T, err error) {
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
func SetAs[T any](ctx ObjectContext, key string, value T) error {
	bytes, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}

	ctx.Set(key, bytes)
	return nil
}

// RunAs helper function runs a run function with specific concrete type as a result
// it does encoding/decoding of bytes automatically using msgpack
func RunAs[T any](ctx Context, fn func(RunContext) (T, error)) (output T, err error) {
	bytes, err := ctx.Run(func(ctx RunContext) ([]byte, error) {
		out, err := fn(ctx)
		if err != nil {
			return nil, err
		}

		bytes, err := msgpack.Marshal(out)
		return bytes, TerminalError(err)
	})

	if err != nil {
		return output, err
	}

	err = msgpack.Unmarshal(bytes, &output)

	return output, TerminalError(err)
}

type Awakeable[T any] interface {
	Id() string
	Result() (T, error)
	futures.Selectable
}

type decodingAwakeable[T any] struct {
	Awakeable[[]byte]
}

func (d decodingAwakeable[T]) Id() string { return d.Awakeable.Id() }
func (d decodingAwakeable[T]) Result() (out T, err error) {
	bytes, err := d.Awakeable.Result()
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(bytes, &out); err != nil {
		return out, err
	}
	return
}

func AwakeableAs[T any](ctx Context) Awakeable[T] {
	return decodingAwakeable[T]{Awakeable: ctx.Awakeable()}
}

func ResolveAwakeableAs[T any](ctx Context, id string, value T) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return TerminalError(err)
	}
	ctx.ResolveAwakeable(id, bytes)
	return nil
}

type After interface {
	Done()
	futures.Selectable
}
