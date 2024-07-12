package rcontext

import "context"

type LogSource int

const (
	LogSourceRestate = iota
	LogSourceUser
)

type LogContext struct {
	Source      LogSource
	IsReplaying bool
}

type logContextKey struct{}

func WithLogContext(parent context.Context, logContext *LogContext) context.Context {
	return context.WithValue(parent, logContextKey{}, logContext)
}

func LogContextFrom(ctx context.Context) *LogContext {
	if val, ok := ctx.Value(logContextKey{}).(*LogContext); ok {
		return val
	}
	return nil
}
