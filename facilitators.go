package restate

import (
	"fmt"
	"time"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/futures"
)

type getOptions struct {
	codec encoding.Codec
}

type GetOption interface {
	beforeGet(*getOptions)
}

// GetAs helper function to get a key as specific type. Note that
// if there is no associated value with key, an error ErrKeyNotFound is
// returned
// it does encoding/decoding of bytes, defaulting to json codec
func GetAs[T any](ctx ObjectSharedContext, key string, options ...GetOption) (output T, err error) {
	opts := getOptions{}
	for _, opt := range options {
		opt.beforeGet(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}

	bytes := ctx.Get(key)
	if bytes == nil {
		// key does not exist.
		return output, ErrKeyNotFound
	}

	if err := opts.codec.Unmarshal(bytes, &output); err != nil {
		return output, TerminalError(fmt.Errorf("failed to unmarshal Get state into T: %w", err))
	}

	return output, nil
}

type setOptions struct {
	codec encoding.Codec
}

type SetOption interface {
	beforeSet(*setOptions)
}

// SetAs helper function to set a key value with a generic type T.
// it does encoding/decoding of bytes automatically, defaulting to json codec
func SetAs(ctx ObjectContext, key string, value any, options ...SetOption) error {
	opts := setOptions{}
	for _, opt := range options {
		opt.beforeSet(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}

	bytes, err := opts.codec.Marshal(value)
	if err != nil {
		return TerminalError(fmt.Errorf("failed to marshal Set value: %w", err))
	}

	ctx.Set(key, bytes)
	return nil
}

type runOptions struct {
	codec encoding.Codec
}

type RunOption interface {
	beforeRun(*runOptions)
}

// RunAs helper function runs a run function with specific concrete type as a result
// it does encoding/decoding of bytes automatically, defaulting to json codec
func RunAs[T any](ctx Context, fn func(RunContext) (T, error), options ...RunOption) (output T, err error) {
	opts := runOptions{}
	for _, opt := range options {
		opt.beforeRun(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}

	bytes, err := ctx.Run(func(ctx RunContext) ([]byte, error) {
		out, err := fn(ctx)
		if err != nil {
			return nil, err
		}

		bytes, err := opts.codec.Marshal(out)
		if err != nil {
			return nil, TerminalError(fmt.Errorf("failed to marshal Run output: %w", err))
		}
		return bytes, nil
	})

	if err != nil {
		return output, err
	}

	if err := opts.codec.Unmarshal(bytes, &output); err != nil {
		return output, TerminalError(fmt.Errorf("failed to unmarshal Run output into T: %w", err))
	}

	return output, nil
}

// Awakeable is the Go representation of a Restate awakeable; a 'promise' to a future
// value or error, that can be resolved or rejected by other services.
type Awakeable[T any] interface {
	// Id returns the awakeable ID, which can be stored or sent to a another service
	Id() string
	// Result blocks on receiving the result of the awakeable, returning the value it was
	// resolved with or the error it was rejected with.
	// It is *not* safe to call this in a goroutine - use Context.Select if you
	// want to wait on multiple results at once.
	Result() (T, error)
	futures.Selectable
}

type decodingAwakeable[T any] struct {
	Awakeable[[]byte]
	opts awakeableOptions
}

func (d decodingAwakeable[T]) Id() string { return d.Awakeable.Id() }
func (d decodingAwakeable[T]) Result() (out T, err error) {
	bytes, err := d.Awakeable.Result()
	if err != nil {
		return out, err
	}
	if err := d.opts.codec.Unmarshal(bytes, &out); err != nil {
		return out, TerminalError(fmt.Errorf("failed to unmarshal Awakeable result into T: %w", err))
	}
	return
}

type awakeableOptions struct {
	codec encoding.Codec
}

type AwakeableOption interface {
	beforeAwakeable(*awakeableOptions)
}

// AwakeableAs helper function to treat awakeable values as a particular type.
// Bytes are deserialised using JSON by default
func AwakeableAs[T any](ctx Context, options ...AwakeableOption) Awakeable[T] {
	opts := awakeableOptions{}
	for _, opt := range options {
		opt.beforeAwakeable(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}
	return decodingAwakeable[T]{ctx.Awakeable(), opts}
}

type resolveAwakeableOptions struct {
	codec encoding.Codec
}

type ResolveAwakeableOption interface {
	beforeResolveAwakeable(*resolveAwakeableOptions)
}

// ResolveAwakeableAs helper function to resolve an awakeable with a particular type
// The type will be serialised to bytes, defaulting to JSON
func ResolveAwakeableAs(ctx Context, id string, value any, options ...ResolveAwakeableOption) error {
	opts := resolveAwakeableOptions{}
	for _, opt := range options {
		opt.beforeResolveAwakeable(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}
	bytes, err := opts.codec.Marshal(value)
	if err != nil {
		return TerminalError(fmt.Errorf("failed to marshal ResolveAwakeable value: %w", err))
	}
	ctx.ResolveAwakeable(id, bytes)
	return nil
}

type callOptions struct {
	codec encoding.Codec
}

type CallOption interface {
	beforeCall(*callOptions)
}

type codecCallClient[O any] struct {
	client  CallClient[[]byte, []byte]
	options callOptions
}

func (c codecCallClient[O]) RequestFuture(input any) (ResponseFuture[O], error) {
	bytes, err := c.options.codec.Marshal(input)
	if err != nil {
		return nil, TerminalError(fmt.Errorf("failed to marshal RequestFuture input: %w", err))
	}
	fut, err := c.client.RequestFuture(bytes)
	if err != nil {
		return nil, err
	}
	return decodingResponseFuture[O]{fut, c.options}, nil
}

func (c codecCallClient[O]) Request(input any) (output O, err error) {
	fut, err := c.RequestFuture(input)
	if err != nil {
		return output, err
	}
	return fut.Response()
}

func (c codecCallClient[O]) Send(input any, delay time.Duration) error {
	bytes, err := c.options.codec.Marshal(input)
	if err != nil {
		return TerminalError(fmt.Errorf("failed to marshal Send input: %w", err))
	}
	return c.client.Send(bytes, delay)
}

// CallAs helper function to use a codec for encoding and decoding, defaulting to JSON
func CallAs[O any](client CallClient[[]byte, []byte], options ...CallOption) CallClient[any, O] {
	opts := callOptions{}
	for _, opt := range options {
		opt.beforeCall(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}
	return codecCallClient[O]{client, opts}
}

// SendAs helper function to use a codec for encoding .Send request parameters, defaulting to JSON
func SendAs(client CallClient[[]byte, []byte], options ...CallOption) SendClient[any] {
	opts := callOptions{}
	for _, opt := range options {
		opt.beforeCall(&opts)
	}
	if opts.codec == nil {
		opts.codec = encoding.JSONCodec{}
	}
	return codecCallClient[struct{}]{client, opts}
}

type decodingResponseFuture[O any] struct {
	ResponseFuture[[]byte]
	options callOptions
}

func (d decodingResponseFuture[O]) Response() (output O, err error) {
	bytes, err := d.ResponseFuture.Response()
	if err != nil {
		return output, err
	}

	if err := d.options.codec.Unmarshal(bytes, &output); err != nil {
		return output, TerminalError(fmt.Errorf("failed to unmarshal Call response into O: %w", err))
	}

	return output, nil
}