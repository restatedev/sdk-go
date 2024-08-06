package main

import (
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
)

const CANCELED_STATE = "canceled"

type BlockingOperation string

const (
	CALL      BlockingOperation = "CALL"
	SLEEP     BlockingOperation = "SLEEP"
	AWAKEABLE BlockingOperation = "AWAKEABLE"
)

func init() {
	REGISTRY.AddRouter(
		restate.NewObjectRouter("CancelTestRunner").
			Handler("startTest", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, operation BlockingOperation) (restate.Void, error) {
					if err := ctx.Object("CancelTestBlockingService", "", "block").Request(operation, restate.Void{}); err != nil {
						if restate.ErrorCode(err) == 409 {
							return restate.Void{}, ctx.Set(CANCELED_STATE, true)
						}
						return restate.Void{}, err
					}
					return restate.Void{}, nil
				})).
			Handler("verifyTest", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
					canceled, err := restate.GetAs[bool](ctx, CANCELED_STATE)
					if err != nil && err != restate.ErrKeyNotFound {
						return false, err
					}
					return canceled, nil
				})))
	REGISTRY.AddRouter(
		restate.NewObjectRouter("CancelTestBlockingService").
			Handler("block", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, operation BlockingOperation) (restate.Void, error) {
					awakeable := ctx.Awakeable()
					if err := ctx.Object("AwakeableHolder", "cancel", "hold").Request(awakeable.Id(), restate.Void{}); err != nil {
						return restate.Void{}, err
					}
					if err := awakeable.Result(restate.Void{}); err != nil {
						return restate.Void{}, err
					}
					switch operation {
					case CALL:
						return restate.Void{}, ctx.Object("CancelTestBlockingService", "", "block").Request(operation, restate.Void{})
					case SLEEP:
						return restate.Void{}, ctx.Sleep(1024 * time.Hour * 24)
					case AWAKEABLE:
						return restate.Void{}, ctx.Awakeable().Result(restate.Void{})
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
