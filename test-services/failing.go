package main

import (
	"fmt"
	"sync/atomic"

	restate "github.com/restatedev/sdk-go"
)

func init() {
	var eventualSuccessCalls atomic.Int32
	var eventualSuccessSideEffectCalls atomic.Int32

	REGISTRY.AddRouter(
		restate.NewObjectRouter("Failing").
			Handler("terminallyFailingCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, errorMessage string) (restate.Void, error) {
					return restate.Void{}, restate.TerminalError(fmt.Errorf(errorMessage))
				})).
			Handler("callTerminallyFailingCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, errorMessage string) (string, error) {
					if err := ctx.Object("Failing", ctx.Rand().UUID().String(), "terminallyFailingCall").Request(errorMessage, restate.Void{}); err != nil {
						return "", err
					}

					return "", restate.TerminalError(fmt.Errorf("This should be unreachable"))
				})).
			Handler("failingCallWithEventualSuccess", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (int32, error) {
					currentAttempt := eventualSuccessCalls.Add(1)
					if currentAttempt >= 4 {
						eventualSuccessCalls.Store(0)
						return currentAttempt, nil
					} else {
						return 0, fmt.Errorf("Failed at attempt: %d", currentAttempt)
					}
				})).
			Handler("failingSideEffectWithEventualSuccess", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (int32, error) {
					return restate.RunAs(ctx, func(ctx restate.RunContext) (int32, error) {
						currentAttempt := eventualSuccessCalls.Add(1)
						if currentAttempt >= 4 {
							eventualSuccessSideEffectCalls.Store(0)
							return currentAttempt, nil
						} else {
							return 0, fmt.Errorf("Failed at attempt: %d", currentAttempt)
						}
					})
				})).
			Handler("terminallyFailingSideEffect", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, errorMessage string) (restate.Void, error) {
					return restate.RunAs(ctx, func(ctx restate.RunContext) (restate.Void, error) {
						return restate.Void{}, restate.TerminalError(fmt.Errorf(errorMessage))
					})
				})))
}
