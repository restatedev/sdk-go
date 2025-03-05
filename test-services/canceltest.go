package main

import (
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
)

const CanceledState = "canceled"

type BlockingOperation string

const (
	CallOp      BlockingOperation = "CALL"
	SleepOp     BlockingOperation = "SLEEP"
	AwakeableOp BlockingOperation = "AWAKEABLE"
)

func init() {
	REGISTRY.AddDefinition(
		restate.NewObject("CancelTestRunner").
			Handler("startTest", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, operation BlockingOperation) (restate.Void, error) {
					if _, err := restate.Object[restate.Void](ctx, "CancelTestBlockingService", restate.Key(ctx), "block").Request(operation); err != nil {
						if restate.ErrorCode(err) == 409 {
							restate.Set(ctx, CanceledState, true)
							return restate.Void{}, nil
						}
						return restate.Void{}, err
					}
					return restate.Void{}, nil
				})).
			Handler("verifyTest", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
					return restate.Get[bool](ctx, CanceledState)
				})))
	REGISTRY.AddDefinition(
		restate.NewObject("CancelTestBlockingService").
			Handler("block", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, operation BlockingOperation) (restate.Void, error) {
					awakeable := restate.Awakeable[restate.Void](ctx)
					if _, err := restate.Object[restate.Void](ctx, "AwakeableHolder", restate.Key(ctx), "hold").Request(awakeable.Id()); err != nil {
						return restate.Void{}, err
					}
					if _, err := awakeable.Result(); err != nil {
						return restate.Void{}, err
					}
					switch operation {
					case CallOp:
						return restate.Object[restate.Void](ctx, "CancelTestBlockingService", restate.Key(ctx), "block").Request(operation)
					case SleepOp:
						return restate.Void{}, restate.Sleep(ctx, 1024*time.Hour*24)
					case AwakeableOp:
						return restate.Awakeable[restate.Void](ctx).Result()
					default:
						return restate.Void{}, restate.TerminalError(fmt.Errorf("unexpected operation %s", operation), 400)
					}
				})).
			Handler("isUnlocked", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					// no-op
					return restate.Void{}, nil
				})))
}
