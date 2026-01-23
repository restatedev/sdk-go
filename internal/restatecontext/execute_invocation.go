package restatecontext

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"

	"github.com/restatedev/sdk-go/internal/errors"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

func ExecuteInvocation(ctx context.Context, logger *slog.Logger, stateMachine *statemachine.StateMachine, conn io.ReadWriteCloser, handler Handler, dropReplayLogs bool, logHandler slog.Handler, attemptHeaders map[string][]string) error {
	// Let's read the input entry
	invocationInput, err := stateMachine.SysInput(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Error when reading invocation input", log.Error(err))
		if err = consumeOutput(ctx, stateMachine, conn); err != nil {
			logger.WarnContext(ctx, "Error when consuming output", log.Error(err))
			return err
		}
		return err
	}

	// Instantiate the restate context
	restateCtx := newContext(ctx, stateMachine, invocationInput, conn, attemptHeaders, dropReplayLogs, logHandler)

	// Invoke the handler
	invoke(restateCtx, handler, logger)
	return nil
}

func invoke(restateCtx *ctx, handler Handler, logger *slog.Logger) {
	// Run read loop on a goroutine
	go func(restateCtx *ctx, logger *slog.Logger) { restateCtx.readInputLoop(logger) }(restateCtx, logger)

	defer func() {
		// recover will return a non-nil object
		// if there was a panic
		//
		recovered := recover()

		switch typ := recovered.(type) {
		case nil:
			// nothing to do, just exit
			break
		case *statemachine.SuspensionError:
		case statemachine.SuspensionError:
			restateCtx.internalLogger.LogAttrs(restateCtx, slog.LevelInfo, "Suspending invocation")
			break
		default:
			restateCtx.internalLogger.LogAttrs(restateCtx, slog.LevelError, "Invocation panicked, returning error to Restate", slog.Any("err", typ))

			if err := restateCtx.stateMachine.NotifyError(restateCtx, fmt.Sprint(typ), string(debug.Stack())); err != nil {
				restateCtx.internalLogger.WarnContext(restateCtx, "Error when notifying error to state restateContext", log.Error(err))
			}

			break
		}

		// Consume all the state restateContext output as last step
		if err := consumeOutput(restateCtx, restateCtx.stateMachine, restateCtx.conn); err != nil {
			restateCtx.internalLogger.WarnContext(restateCtx, "Error when consuming output", log.Error(err))
		}
	}()

	restateCtx.internalLogger.InfoContext(restateCtx, "Handling invocation")

	var bytes []byte
	var err error
	bytes, err = handler.Call(restateCtx, restateCtx.request.Body)

	if err != nil && errors.IsTerminalError(err) {
		restateCtx.internalLogger.LogAttrs(restateCtx, slog.LevelWarn, "Invocation returned a terminal failure", log.Error(err))

		failure := pbinternal.Failure{}
		failure.SetCode(uint32(errors.ErrorCode(err)))
		failure.SetMessage(err.Error())
		outputParameters := pbinternal.VmSysWriteOutputParameters{}
		outputParameters.SetFailure(&failure)
		outputParameters.SetNonDeterministicSerialization(isNonDeterministicCodec(handler.GetOptions().Codec))
		if err := restateCtx.stateMachine.SysWriteOutput(restateCtx, &outputParameters); err != nil {
			// This is handled by the panic catcher above
			panic(err)
		}
	} else if err != nil {
		restateCtx.internalLogger.LogAttrs(restateCtx, slog.LevelWarn, "Invocation returned a non-terminal failure", log.Error(err))

		// This is handled by the panic catcher above
		panic(err)
	} else {
		restateCtx.internalLogger.InfoContext(restateCtx, "Invocation completed successfully")

		outputParameters := pbinternal.VmSysWriteOutputParameters{}
		outputParameters.SetSuccess(bytes)
		outputParameters.SetNonDeterministicSerialization(isNonDeterministicCodec(handler.GetOptions().Codec))
		if err := restateCtx.stateMachine.SysWriteOutput(restateCtx, &outputParameters); err != nil {
			// This is handled by the panic catcher above
			panic(err)
		}
	}

	// Sys_end the state restateContext
	if err := restateCtx.stateMachine.SysEnd(restateCtx); err != nil {
		// This is handled by the panic catcher above
		panic(err)
	}
}
