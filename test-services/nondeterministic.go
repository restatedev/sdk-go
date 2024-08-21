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
		countKey := restate.Key(ctx)
		invocationCountsMtx.Lock()
		defer invocationCountsMtx.Unlock()

		invocationCounts[countKey] += 1
		return invocationCounts[countKey]%2 == 1
	}
	incrementCounter := func(ctx restate.ObjectContext) {
		restate.ObjectSend(ctx, "Counter", restate.Key(ctx), "add").Send(int64(1))
	}

	REGISTRY.AddDefinition(
		restate.NewObject("NonDeterministic").
			Handler("eitherSleepOrCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						restate.Sleep(ctx, 100*time.Millisecond)
					} else {
						if _, err := restate.Object[restate.Void](ctx, "Counter", "abc", "get").Request(restate.Void{}); err != nil {
							return restate.Void{}, err
						}
					}

					// This is required to cause a suspension after the non-deterministic operation
					restate.Sleep(ctx, 100*time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})).
			Handler("callDifferentMethod", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						if _, err := restate.Object[restate.Void](ctx, "Counter", "abc", "get").Request(restate.Void{}); err != nil {
							return restate.Void{}, err
						}
					} else {
						if _, err := restate.Object[restate.Void](ctx, "Counter", "abc", "reset").Request(restate.Void{}); err != nil {
							return restate.Void{}, err
						}
					}

					// This is required to cause a suspension after the non-deterministic operation
					restate.Sleep(ctx, 100*time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})).
			Handler("backgroundInvokeWithDifferentTargets", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						restate.ObjectSend(ctx, "Counter", "abc", "get").Send(restate.Void{})
					} else {
						restate.ObjectSend(ctx, "Counter", "abc", "reset").Send(restate.Void{})
					}

					// This is required to cause a suspension after the non-deterministic operation
					restate.Sleep(ctx, 100*time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})).
			Handler("setDifferentKey", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						restate.Set(ctx, STATE_A, "my-state")
					} else {
						restate.Set(ctx, STATE_B, "my-state")
					}

					// This is required to cause a suspension after the non-deterministic operation
					restate.Sleep(ctx, 100*time.Millisecond)
					incrementCounter(ctx)
					return restate.Void{}, nil
				})))
}
