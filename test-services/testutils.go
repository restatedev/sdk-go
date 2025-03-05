package main

import (
	"os"
	"strings"
	"sync/atomic"
	"time"

	restate "github.com/restatedev/sdk-go"
)

func init() {
	REGISTRY.AddDefinition(
		restate.NewService("TestUtilsService").
			Handler("echo", restate.NewServiceHandler(
				func(ctx restate.Context, input string) (string, error) {
					return input, nil
				})).
			Handler("uppercaseEcho", restate.NewServiceHandler(
				func(ctx restate.Context, input string) (string, error) {
					return strings.ToUpper(input), nil
				})).
			Handler("echoHeaders", restate.NewServiceHandler(
				func(ctx restate.Context, _ restate.Void) (map[string]string, error) {
					return ctx.Request().Headers, nil
				})).
			Handler("rawEcho", restate.NewServiceHandler(
				func(ctx restate.Context, input []byte) ([]byte, error) {
					return input, nil
				}, restate.WithBinary)).
			Handler("sleepConcurrently", restate.NewServiceHandler(
				func(ctx restate.Context, millisDuration []int64) (restate.Void, error) {
					timers := make([]restate.Selectable, 0, len(millisDuration))
					for _, d := range millisDuration {
						timers = append(timers, restate.After(ctx, time.Duration(d)*time.Millisecond))
					}
					selector := restate.Select(ctx, timers...)
					i := 0
					for selector.Remaining() {
						_ = selector.Select()
						i++
					}
					if i != len(timers) {
						return restate.Void{}, restate.TerminalErrorf("unexpected number of timers fired: %d", i)
					}
					return restate.Void{}, nil
				})).
			Handler("countExecutedSideEffects", restate.NewServiceHandler(
				func(ctx restate.Context, increments int32) (int32, error) {
					invokedSideEffects := atomic.Int32{}
					for i := int32(0); i < increments; i++ {
						restate.Run(ctx, func(ctx restate.RunContext) (int32, error) {
							return invokedSideEffects.Add(1), nil
						})
					}
					return invokedSideEffects.Load(), nil
				})).
			Handler("getEnvVariable", restate.NewServiceHandler(getEnvVariable)).
			Handler("cancelInvocation", restate.NewServiceHandler(
				func(ctx restate.Context, invocationId string) (string, error) {
					return "", restate.TerminalErrorf("Cancel invocation not supported yet")
				})),
	)
}

func getEnvVariable(ctx restate.Context, envName string) (string, error) {
	return restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		return os.Getenv(envName), nil
	})
}
