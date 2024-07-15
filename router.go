package restate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/rand"
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
	// Response blocks on the response to the call and unmarshals it into output
	// It is *not* safe to call this in a goroutine - use Context.Selector if you
	// want to wait on multiple results at once.
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

	// Rand returns a random source which will give deterministic results for a given invocation
	// The source wraps the stdlib rand.Rand but with some extra helper methods
	// This source is not safe for use inside .Run()
	Rand() *rand.Rand

	// Sleep for the duration d
	Sleep(d time.Duration)
	// After is an alternative to Context.Sleep which allows you to complete other tasks concurrently
	// with the sleep. This is particularly useful when combined with Context.Selector to race between
	// the sleep and other Selectable operations.
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

	// Run runs the function (fn), storing final results (including terminal errors)
	// durably in the journal, or otherwise for transient errors stopping execution
	// so Restate can retry the invocation. Replays will produce the same value, so
	// all non-deterministic operations (eg, generating a unique ID) *must* happen
	// inside Run blocks.
	// Note: use the RunAs helper function to serialise non-[]byte return values
	Run(fn func(RunContext) ([]byte, error)) ([]byte, error)

	// Awakeable returns a Restate awakeable; a 'promise' to a future
	// value or error, that can be resolved or rejected by other services.
	// Note: use the AwakeableAs helper function to deserialise the []byte value
	Awakeable() Awakeable[[]byte]
	// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
	// resolved with a particular value.
	// Note: use the ResolveAwakeableAs helper function to provide a value to be serialised
	ResolveAwakeable(id string, value []byte)
	// ResolveAwakeable allows an awakeable (not necessarily from this service) to be
	// rejected with a particular error.
	RejectAwakeable(id string, reason error)

	// Selector returns an iterator over blocking Restate operations (sleep, call, awakeable)
	// which allows you to safely run them in parallel. The Selector will store the order
	// that things complete in durably inside Restate, so that on replay the same order
	// can be used. This avoids non-determinism. It is *not* safe to use goroutines or channels
	// outside of Context.Run functions, as they do not behave deterministically.
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
	InputPayload() *encoding.InputPayload
	OutputPayload() *encoding.OutputPayload
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
	// Key retrieves the key for this virtual object invocation. This is a no-op and is
	// always safe to call.
	Key() string
}

// ServiceHandlerFn signature of service (unkeyed) handler function
type ServiceHandlerFn[I any, O any] func(ctx Context, input I) (output O, err error)

// ObjectHandlerFn signature for object (keyed) handler function
type ObjectHandlerFn[I any, O any] func(ctx ObjectContext, input I) (output O, err error)

type Decoder[I any] interface {
	InputPayload() *encoding.InputPayload
	Decode(data []byte) (input I, err error)
}

type Encoder[O any] interface {
	OutputPayload() *encoding.OutputPayload
	Encode(output O) ([]byte, error)
}

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
// it does encoding/decoding of bytes automatically using json
func GetAs[T any](ctx ObjectContext, key string) (output T, err error) {
	bytes, err := ctx.Get(key)
	if err != nil {
		return output, err
	}

	if bytes == nil {
		// key does not exit.
		return output, ErrKeyNotFound
	}

	err = json.Unmarshal(bytes, &output)

	return
}

// SetAs helper function to set a key value with a generic type T.
// it does encoding/decoding of bytes automatically using json
func SetAs[T any](ctx ObjectContext, key string, value T) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	ctx.Set(key, bytes)
	return nil
}

// RunAs helper function runs a run function with specific concrete type as a result
// it does encoding/decoding of bytes automatically using json
func RunAs[T any](ctx Context, fn func(RunContext) (T, error)) (output T, err error) {
	bytes, err := ctx.Run(func(ctx RunContext) ([]byte, error) {
		out, err := fn(ctx)
		if err != nil {
			return nil, err
		}

		bytes, err := json.Marshal(out)
		return bytes, TerminalError(err)
	})

	if err != nil {
		return output, err
	}

	err = json.Unmarshal(bytes, &output)

	return output, TerminalError(err)
}

// Awakeable is the Go representation of a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
type Awakeable[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, returning the value it was
	// resolved with or the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Selector if you
	// want to wait on multiple results at once.
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

// AwakeableAs helper function to treat awakeable values as a particular type.
// Bytes are deserialised as JSON
func AwakeableAs[T any](ctx Context) Awakeable[T] {
	return decodingAwakeable[T]{Awakeable: ctx.Awakeable()}
}

// ResolveAwakeableAs helper function to resolve an awakeable with a particular type
// The type will be serialised to bytes using JSON
func ResolveAwakeableAs[T any](ctx Context, id string, value T) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return TerminalError(err)
	}
	ctx.ResolveAwakeable(id, bytes)
	return nil
}

// After is a handle on a Sleep operation which allows you to do other work concurrently
// with the sleep.
type After interface {
	// Done blocks waiting on the remaining duration of the sleep.
	// It is *not* safe to call this in a goroutine - use Context.Selector if you
	// want to wait on multiple results at once.
	Done()
	futures.Selectable
}
