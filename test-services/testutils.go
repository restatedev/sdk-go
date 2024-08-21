package main

import (
	"strings"
	"sync/atomic"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/interfaces"
)

type CreateAwakeableAndAwaitItRequest struct {
	AwakeableKey string `json:"awakeableKey"`
	// If not null, then await it with orTimeout()
	AwaitTimeout *int64 `json:"awaitTimeout,omitempty"`
}

type CreateAwakeableAndAwaitItResponse struct {
	// timeout or result
	Type string `json:"type"`
	// only present in result case
	Value *string `json:"value,omitempty"`
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
			Handler("createAwakeableAndAwaitIt", restate.NewServiceHandler(
				func(ctx restate.Context, req CreateAwakeableAndAwaitItRequest) (CreateAwakeableAndAwaitItResponse, error) {
					awakeable := restate.Awakeable[string](ctx)
					if _, err := restate.Object[restate.Void](ctx, "AwakeableHolder", req.AwakeableKey, "hold").Request(awakeable.Id()); err != nil {
						return CreateAwakeableAndAwaitItResponse{}, err
					}

					if req.AwaitTimeout == nil {
						result, err := awakeable.Result()
						if err != nil {
							return CreateAwakeableAndAwaitItResponse{}, err
						}
						return CreateAwakeableAndAwaitItResponse{
							Type:  "result",
							Value: &result,
						}, nil
					}

					timeout := restate.After(ctx, time.Duration(*req.AwaitTimeout)*time.Millisecond)
					selector := restate.Select(ctx, timeout, awakeable)
					switch selector.Select() {
					case timeout:
						return CreateAwakeableAndAwaitItResponse{Type: "timeout"}, nil
					case awakeable:
						result, err := awakeable.Result()
						if err != nil {
							return CreateAwakeableAndAwaitItResponse{}, err
						}
						return CreateAwakeableAndAwaitItResponse{Type: "result", Value: &result}, nil
					default:
						panic("unreachable")
					}
				})).
			Handler("sleepConcurrently", restate.NewServiceHandler(
				func(ctx restate.Context, millisDuration []int64) (restate.Void, error) {
					timers := make([]interfaces.Selectable, 0, len(millisDuration))
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
				})))
}
