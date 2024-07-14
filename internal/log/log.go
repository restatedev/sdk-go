package log

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync/atomic"

	"github.com/restatedev/sdk-go/rcontext"
)

const (
	LevelTrace slog.Level = -8
)

type typeValue struct{ inner any }

func (t typeValue) LogValue() slog.Value {
	return slog.StringValue(reflect.TypeOf(t.inner).String())
}

func Type(key string, value any) slog.Attr {
	return slog.Any(key, typeValue{value})
}

type stringerValue[T fmt.Stringer] struct{ inner T }

func (t stringerValue[T]) LogValue() slog.Value {
	return slog.StringValue(t.inner.String())
}

func Stringer[T fmt.Stringer](key string, value T) slog.Attr {
	return slog.Any(key, stringerValue[T]{value})
}

func Error(err error) slog.Attr {
	return slog.String("err", err.Error())
}

type contextInjectingHandler struct {
	logContext *atomic.Pointer[rcontext.LogContext]
	dropReplay bool
	inner      slog.Handler
}

func NewUserContextHandler(logContext *atomic.Pointer[rcontext.LogContext], dropReplay bool, inner slog.Handler) slog.Handler {
	return &contextInjectingHandler{logContext, dropReplay, inner}
}

func NewRestateContextHandler(inner slog.Handler) slog.Handler {
	logContext := atomic.Pointer[rcontext.LogContext]{}
	logContext.Store(&rcontext.LogContext{Source: rcontext.LogSourceRestate, IsReplaying: false})
	return &contextInjectingHandler{&logContext, false, inner}
}

func (d *contextInjectingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	lc := d.logContext.Load()
	if d.dropReplay && lc.IsReplaying {
		return false
	}
	return d.inner.Enabled(rcontext.WithLogContext(ctx, lc), l)
}

func (d *contextInjectingHandler) Handle(ctx context.Context, record slog.Record) error {
	return d.inner.Handle(rcontext.WithLogContext(ctx, d.logContext.Load()), record)
}

func (d *contextInjectingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextInjectingHandler{d.logContext, d.dropReplay, d.inner.WithAttrs(attrs)}
}

func (d *contextInjectingHandler) WithGroup(name string) slog.Handler {
	return &contextInjectingHandler{d.logContext, d.dropReplay, d.inner.WithGroup(name)}
}

var _ slog.Handler = &contextInjectingHandler{}

type dropReplayHandler struct {
	isReplaying func() bool
	inner       slog.Handler
}
