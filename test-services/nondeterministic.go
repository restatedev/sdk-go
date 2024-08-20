package main

import (
	"sync"
	"time"

	restate "github.com/restatedev/sdk-go"
)

const STATE_A = "a"
const STATE_B = "b"

func init() {
	invocationCounts := map[string]int32{}
	invocationCountsMtx := sync.RWMutex{}

	doLeftAction := func(ctx restate.ObjectContext) bool {
		countKey := ctx.Key()
		invocationCountsMtx.Lock()
		defer invocationCountsMtx.Unlock()

		invocationCounts[countKey] += 1
		return invocationCounts[countKey]%2 == 1
	}
	incrementCounter := func(ctx restate.ObjectContext) {
		ctx.Object("Counter", ctx.Key(), "add").Send(int64(1), 0)
	}

	REGISTRY.AddDefinition(
		restate.NewObject("NonDeterministic").
			Handler("eitherSleepOrCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						ctx.Sleep(100 * time.Millisecond)
					} else {
						if err := ctx.Object("Counter", "abc", "get").Request(restate.Void{}, restate.Void{}); err != nil {
							return restate.Void{}, err
						}
					}

					// This is required to cause a suspension after the non-deterministic operation
					ctx.Sleep(100 * time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})).
			Handler("callDifferentMethod", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						if err := ctx.Object("Counter", "abc", "get").Request(restate.Void{}, restate.Void{}); err != nil {
							return restate.Void{}, err
						}
					} else {
						if err := ctx.Object("Counter", "abc", "reset").Request(restate.Void{}, restate.Void{}); err != nil {
							return restate.Void{}, err
						}
					}

					// This is required to cause a suspension after the non-deterministic operation
					ctx.Sleep(100 * time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})).
			Handler("backgroundInvokeWithDifferentTargets", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						ctx.Object("Counter", "abc", "get").Send(restate.Void{}, 0)
					} else {
						ctx.Object("Counter", "abc", "reset").Send(restate.Void{}, 0)
					}

					// This is required to cause a suspension after the non-deterministic operation
					ctx.Sleep(100 * time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})).
			Handler("setDifferentKey", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						ctx.Set(STATE_A, "my-state")
					} else {
						ctx.Set(STATE_B, "my-state")
					}

					// This is required to cause a suspension after the non-deterministic operation
					ctx.Sleep(100 * time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})))
}
