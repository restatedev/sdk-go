package statemachine

import (
	"context"
	"log/slog"
)

type loggerKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

func getLogger(ctx context.Context) *slog.Logger {
	val, _ := ctx.Value(loggerKey{}).(*slog.Logger)
	return val
}
