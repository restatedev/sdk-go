package restate

import (
	"time"

	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

// SleepOption is an option for [Sleep] and [After].
type SleepOption = options.SleepOption

// Sleep for the duration d. Can return a terminal error in the case where the invocation was cancelled mid-sleep.
func Sleep(ctx Context, d time.Duration, opts ...options.SleepOption) TerminalError {
	return ctx.inner().Sleep(d, opts...)
}

// After is an alternative to [Sleep] which allows you to complete other tasks concurrently
// with the sleep. This is particularly useful when combined with [WaitFirst] to race between
// the sleep and other [Future] operations.
func After(ctx Context, d time.Duration, opts ...options.SleepOption) AfterFuture {
	return ctx.inner().After(d, opts...)
}

// AfterFuture is returned by the After operation which allows you to do other work concurrently
// with the sleep.
type AfterFuture = restatecontext.AfterFuture
