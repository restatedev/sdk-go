package main

import (
	"os"
	"strings"
	"sync/atomic"

	restate "github.com/restatedev/sdk-go"
)

type ResolveSignalRequest struct {
	InvocationID string `json:"invocationId"`
	SignalName   string `json:"signalName"`
	Value        string `json:"value"`
}

type RejectSignalRequest struct {
	InvocationID string `json:"invocationId"`
	SignalName   string `json:"signalName"`
	Reason       string `json:"reason"`
}

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
				func(ctx restate.Context, invocationId string) (restate.Void, error) {
					restate.CancelInvocation(ctx, invocationId)
					return restate.Void{}, nil
				})).
			Handler("resolveSignal", restate.NewServiceHandler(
				func(ctx restate.Context, req ResolveSignalRequest) (restate.Void, error) {
					restate.ResolveSignal(ctx, req.InvocationID, req.SignalName, req.Value)
					return restate.Void{}, nil
				})).
			Handler("rejectSignal", restate.NewServiceHandler(
				func(ctx restate.Context, req RejectSignalRequest) (restate.Void, error) {
					restate.RejectSignal(ctx, req.InvocationID, req.SignalName, restate.TerminalErrorf("%s", req.Reason))
					return restate.Void{}, nil
				})),
	)
}

func getEnvVariable(ctx restate.Context, envName string) (string, error) {
	return restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		return os.Getenv(envName), nil
	})
}
