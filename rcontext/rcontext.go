package rcontext

import "context"

// LogSource is an enum to describe the source of a logline
type LogSource int

const (
	// LogSourceRestate logs come from the sdk-go library
	LogSourceRestate = iota
	// LogSourceUser logs come from user handlers that use the Context.Log() logger.
	LogSourceUser
)

// LogContext contains information stored in the context that is passed to loggers
type LogContext struct {
	// The source of the logline
	Source LogSource
	// Whether the user code is currently replaying
	IsReplaying bool
}

type logContextKey struct{}

// WithLogContext stores a [LogContext] in the provided [context.Context], returning a new context
func WithLogContext(parent context.Context, logContext *LogContext) context.Context {
	return context.WithValue(parent, logContextKey{}, logContext)
}

// LogContextFrom retrieves the [LogContext] stored in this [context.Context], or otherwise returns nil
func LogContextFrom(ctx context.Context) *LogContext {
	if val, ok := ctx.Value(logContextKey{}).(*LogContext); ok {
		return val
	}
	return nil
}
