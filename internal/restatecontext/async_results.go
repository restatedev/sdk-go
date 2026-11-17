package restatecontext

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

var CancelledFailureValue = func() statemachine.Value {
	failure := pbinternal.TerminalFailure{}
	failure.SetCode(409)
	failure.SetMessage("Cancelled")
	return statemachine.ValueFailure{Failure: &failure}
}()

func errorFromFailure(failure statemachine.ValueFailure) errors.TerminalError {
	return errors.NewTerminalError(
		failure.Failure.GetMessage(),
		errors.WithCode(errors.Code(failure.Failure.GetCode())),
		errors.WithMetadata(metadataFromHeaders(failure.Failure.GetMetadata())),
	)
}

type Future interface {
	handle() uint32
	isResolved() bool
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

func (a *asyncResult) isResolved() bool {
	return a.result.Load() != nil
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

// maxDrainedInputChunks bounds how many chunks drainAvailableInput feeds to the core in one go, so
// that input arriving back to back cannot hold off DoProgress (and the handler) indefinitely.
const maxDrainedInputChunks = 32

// handleInput feeds one receive from readChan to the core. ok is false when readChan was closed.
func (restateCtx *ctx) handleInput(readRes readResult, ok bool) {
	if !ok {
		// readInputLoop is the only sender on readChan and closes it when the input stream hits
		// EOF, so input is closed for the rest of the invocation. Notify the core, then set
		// readChan to nil to disable the receive in pollProgress: a closed channel is permanently
		// ready to receive, so leaving it armed makes that select spin the enclosing loop instead
		// of waiting. Receiving from a nil channel blocks forever, which leaves a run completion
		// and context cancellation as the only wakeups. Those suffice because the core reports
		// DoProgressWaitingExternalProgress only while something external is still pending; with
		// input closed and nothing pending it suspends.
		if err := restateCtx.stateMachine.NotifyInputClosed(restateCtx); err != nil {
			panic(err)
		}
		restateCtx.readChan = nil
		return
	}
	// NotifyInput copies the bytes into the core, so the buffer can be reused right away.
	err := restateCtx.stateMachine.NotifyInput(restateCtx, readRes.buf[0:readRes.nRead])
	BufPool.Put(readRes.buf)
	if err != nil {
		panic(err)
	}
}

// drainAvailableInput feeds the core the input readInputLoop has already read, without waiting for
// more. It stops when no chunk is ready, after the channel close, or after maxDrainedInputChunks.
func (restateCtx *ctx) drainAvailableInput() {
	for i := 0; i < maxDrainedInputChunks && restateCtx.readChan != nil; i++ {
		select {
		case readRes, ok := <-restateCtx.readChan:
			restateCtx.handleInput(readRes, ok)
		default:
			return
		}
	}
}

func (restateCtx *ctx) pollProgress(handles []uint32) bool {
	// Pump output once
	if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.stream); err != nil {
		panic(err)
	}

	for {
		// Hand the core all input that has already arrived before asking it for progress. Every
		// DoProgress that ends up waiting re-sends AwaitingOnMessage, so doing this once per batch of
		// input rather than once per chunk avoids redundant ones when a message spans several reads.
		// DoProgress must still follow every batch: we can't tell whether a batch completed a
		// message, and Restate relies on a fresh AwaitingOnMessage after e.g. a run proposal ack.
		restateCtx.drainAvailableInput()

		progressResult, err := restateCtx.stateMachine.DoProgress(restateCtx, handles)
		if err != nil {
			panic(err)
		}
		if _, ok := progressResult.(statemachine.DoProgressAnyCompleted); ok {
			return false
		}
		if _, ok := progressResult.(statemachine.DoProgressWaitingExternalProgress); ok {
			// Write out awaiting on message
			if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.stream); err != nil {
				panic(err)
			}

			// Either wait for at least one read or for run proposals
			select {
			case readRes, ok := <-restateCtx.readChan:
				// Whatever else is already waiting on readChan is fed to the core by
				// drainAvailableInput at the top of the loop, before the next DoProgress.
				restateCtx.handleInput(readRes, ok)
				break
			case proposal := <-restateCtx.runClosureCompletions:
				// Propose completion
				if err := restateCtx.stateMachine.ProposeRunCompletion(restateCtx, proposal); err != nil {
					panic(err)
				}

				// Pump output once. This is needed for the run completion to be effectively written
				if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.stream); err != nil {
					panic(err)
				}
				break
			case <-restateCtx.Done():
				panic(restateCtx.Err())
			}
		}
		if _, ok := progressResult.(statemachine.DoProgressCancelSignalReceived); ok {
			// Pump output once. This is needed for cancel commands to be effectively written
			if err := takeOutputAndWriteOut(restateCtx, restateCtx.stateMachine, restateCtx.stream); err != nil {
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
