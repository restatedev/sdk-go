package restatecontext

import (
	"fmt"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"time"
)

func (restateCtx *ctx) Sleep(d time.Duration, opts ...options.SleepOption) error {
	return restateCtx.After(d, opts...).Done()
}

// After is a coreHandle on a Sleep operation which allows you to do other work concurrently
// with the sleep.
type AfterFuture interface {
	// Done blocks waiting on the remaining duration of the sleep.
	// It is *not* safe to call this in a goroutine - use Context.Select if you want to wait on multiple
	// results at once. Can return a terminal error in the case where the invocation was cancelled mid-sleep,
	// hence Done() should always be called, even afterFuture using Context.Select.
	Done() error
	Selectable
}

func (restateCtx *ctx) After(d time.Duration, opts ...options.SleepOption) AfterFuture {
	o := options.SleepOptions{}
	for _, opt := range opts {
		opt.BeforeSleep(&o)
	}

	handle, err := restateCtx.stateMachine.SysSleep(restateCtx, o.Name, d)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	return &afterFuture{
		asyncResult: newAsyncResult(restateCtx, handle),
	}
}

type afterFuture struct {
	asyncResult
}

func (a *afterFuture) Done() error {
	switch result := a.pollProgressAndLoadValue().(type) {
	case statemachine.ValueVoid:
		return nil
	case statemachine.ValueFailure:
		return errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}
