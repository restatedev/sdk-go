package main

import (
	"fmt"
	"sync/atomic"
	"time"

	restate "github.com/restatedev/sdk-go"
)

func init() {
	var eventualSuccessCalls atomic.Int32
	var eventualSuccessSideEffectCalls atomic.Int32
	var eventualFailureSideEffectCalls atomic.Int32

	REGISTRY.AddDefinition(
		restate.NewObject("Failing").
			Handler("terminallyFailingCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, errorMessage string) (restate.Void, error) {
					return restate.Void{}, restate.TerminalErrorf("%s", errorMessage)
				})).
			Handler("callTerminallyFailingCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, errorMessage string) (string, error) {
					if _, err := restate.Object[restate.Void](ctx, "Failing", restate.RandUUID(ctx).String(), "terminallyFailingCall").Request(errorMessage); err != nil {
						return "", err
					}

					return "", restate.TerminalErrorf("This should be unreachable")
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
			Handler("terminallyFailingSideEffect", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, errorMessage string) (restate.Void, error) {
					return restate.Run(ctx, func(ctx restate.RunContext) (restate.Void, error) {
						return restate.Void{}, restate.TerminalErrorf("%s", errorMessage)
					})
				})).
			Handler("sideEffectSucceedsAfterGivenAttempts", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, minimumAttempts int32) (int32, error) {
					return restate.Run(ctx, func(ctx restate.RunContext) (int32, error) {
						currentAttempt := eventualSuccessSideEffectCalls.Add(1)
						if currentAttempt >= minimumAttempts {
							eventualSuccessSideEffectCalls.Store(0)
							return currentAttempt, nil
						} else {
							return 0, fmt.Errorf("Failed at attempt: %d", currentAttempt)
						}
					},
						restate.WithName("failing_side_effect"),
						restate.WithInitialRetryInterval(time.Millisecond*10),
						restate.WithRetryIntervalFactor(1.0))
				})).
			Handler("sideEffectFailsAfterGivenAttempts", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, retryPolicyMaxRetryCount uint) (int32, error) {
					_, err := restate.Run(ctx, func(ctx restate.RunContext) (int32, error) {
						currentAttempt := eventualFailureSideEffectCalls.Add(1)
						return 0, fmt.Errorf("Failed at attempt: %d", currentAttempt)
					}, restate.WithName("failing_side_effect"), restate.WithInitialRetryInterval(time.Millisecond*10), restate.WithRetryIntervalFactor(1.0), restate.WithMaxRetryAttempts(retryPolicyMaxRetryCount))
					if err != nil {
						return eventualFailureSideEffectCalls.Load(), nil
					}
					return 0, restate.TerminalErrorf("Expecting the side effect to fail!")
				})))
}
