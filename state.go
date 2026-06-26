package restate

import (
	"github.com/restatedev/sdk-go/internal/options"
)

// GetOption is an option for [Get].
type GetOption = options.GetOption

// SetOption is an option for [Set].
type SetOption = options.SetOption

// Get gets the value for a key. If there is no associated value with key, the zero value is returned.
// To check explicitly for this case pass a pointer eg *string as T.
// If the invocation was cancelled while obtaining the state (only possible if eager state is disabled),
// a cancellation error is returned.
func Get[T any](ctx ObjectSharedContext, key string, options ...options.GetOption) (output T, err TerminalError) {
	_, err = ctx.inner().Get(key, &output, options...)
	return output, err
}

// Keys retrieves all the state keys set inside a virtual object instance.
func Keys(ctx ObjectSharedContext) ([]string, TerminalError) {
	return ctx.inner().Keys()
}

// Set sets a value against a key, using the provided codec (defaults to JSON)
func Set[T any](ctx ObjectContext, key string, value T, options ...options.SetOption) {
	ctx.inner().Set(key, value, options...)
}

// Clear deletes a key
func Clear(ctx ObjectContext, key string) {
	ctx.inner().Clear(key)
}

// ClearAll drops all stored state associated with this Object key
func ClearAll(ctx ObjectContext) {
	ctx.inner().ClearAll()
}
