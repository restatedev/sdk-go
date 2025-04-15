package restatecontext

import (
	"fmt"
	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"io"
	"sync"
	"sync/atomic"
)

var CancelledFailureValue = func() statemachine.Value {
	failure := pbinternal.Failure{}
	failure.SetCode(409)
	failure.SetMessage("Cancelled")
	return statemachine.ValueFailure{Failure: &failure}
}()

func errorFromFailure(failure statemachine.ValueFailure) error {
	return &errors.CodeError{Inner: &errors.TerminalError{Inner: fmt.Errorf(failure.Failure.GetMessage())}, Code: errors.Code(failure.Failure.GetCode())}
}

type Selectable interface {
	handle() uint32
}

type asyncResult struct {
	ctx        *ctx
	coreHandle uint32
	poll       sync.Once
	result     atomic.Value // statemachine.Value
}

func newAsyncResult(ctx *ctx, handle uint32) asyncResult {
	return asyncResult{
		ctx:        ctx,
		coreHandle: handle,
	}
}

func (a *asyncResult) handle() uint32 {
	return a.coreHandle
}

func (a *asyncResult) pollProgress() {
	if a.result.Load() != nil {
		return
	}
	a.poll.Do(func() {
		cancelled := a.ctx.pollProgress([]uint32{a.coreHandle})
		if cancelled {
			a.result.Store(CancelledFailureValue)
		} else {
			value, err := a.ctx.stateMachine.TakeNotification(a.ctx, a.coreHandle)
			if value == nil {
				panic("The value should not be nil anymore")
			}
			if err != nil {
				panic(err)
			}
			a.result.Store(value)
		}
	})
}

func (a *asyncResult) mustLoadValue() statemachine.Value {
	value := a.result.Load()
	if value == nil {
		panic("value is not expected to be nil at this point")
	}
	return value.(statemachine.Value)
}

func (a *asyncResult) pollProgressAndLoadValue() statemachine.Value {
	a.pollProgress()
	return a.mustLoadValue()
}

func (restateCtx *ctx) pollProgress(handles []uint32) bool {
	// Pump output once
	if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.conn); err != nil {
		panic(err)
	}

	for {
		progressResult, err := restateCtx.stateMachine.DoProgress(restateCtx, handles)
		if err != nil {
			panic(err)
		}
		if _, ok := progressResult.(statemachine.DoProgressAnyCompleted); ok {
			return false
		}
		_, isPendingRun := progressResult.(statemachine.DoProgressWaitingPendingRun)
		_, isReadFromInput := progressResult.(statemachine.DoProgressReadFromInput)
		if isPendingRun || isReadFromInput {
			// Either wait for at least one read or for run proposals
			select {
			case readRes := <-restateCtx.readChan:
				if readRes.err == io.EOF {
					// Got EOF, notify and break
					if err = restateCtx.stateMachine.NotifyInputClosed(restateCtx); err != nil {
						panic(err)
					}
					break
				} else if err != nil {
					// Cannot read input anymore
					panic(fmt.Errorf("error when reading the input stream %e", err))
				}
				if err = restateCtx.stateMachine.NotifyInput(restateCtx, readRes.buf[0:readRes.nRead]); err != nil {
					panic(err)
				}
				BufPool.Put(readRes.buf)
				break
			case proposal := <-restateCtx.runClosureCompletions:
				// Propose completion
				if err := restateCtx.stateMachine.ProposeRunCompletion(restateCtx, proposal); err != nil {
					panic(err)
				}

				// Pump output once. This is needed for the run completion to be effectively written
				if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.conn); err != nil {
					panic(err)
				}
				break
			case <-restateCtx.Done():
				panic(restateCtx.Err())
			}
		}
		if _, ok := progressResult.(statemachine.DoProgressCancelSignalReceived); ok {
			// Pump output once. This is needed for cancel commands to be effectively written
			if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.conn); err != nil {
				panic(err)
			}

			return true
		}
		if executeRun, ok := progressResult.(statemachine.DoProgressExecuteRun); ok {
			closure, ok := restateCtx.runClosures[executeRun.Handle]
			if !ok {
				panic(fmt.Sprintf("Need to run a Run closure with coreHandle %d, but it doesn't exist. This is an SDK bug.", executeRun.Handle))
			}

			// Delete this closure from the running list
			delete(restateCtx.runClosures, executeRun.Handle)

			// Run closure in a separate goroutine, proposing the result to runClosureCompletions
			go func(runClosureCompletions chan *pbinternal.VmProposeRunCompletionParameters, closure func() *pbinternal.VmProposeRunCompletionParameters) {
				runClosureCompletions <- closure()
			}(restateCtx.runClosureCompletions, closure)
		}
	}
}
