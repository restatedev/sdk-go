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
	incrementCounter := func(ctx restate.ObjectContext) error {
		return ctx.Object("Counter", ctx.Key(), "add").Send(int64(1), 0)
	}

	REGISTRY.AddRouter(
		restate.NewObjectRouter("NonDeterministic").
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
					return restate.Void{}, incrementCounter(ctx)
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
					return restate.Void{}, incrementCounter(ctx)
				})).
			Handler("backgroundInvokeWithDifferentTargets", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						if err := ctx.Object("Counter", "abc", "get").Send(restate.Void{}, 0); err != nil {
							return restate.Void{}, err
						}
					} else {
						if err := ctx.Object("Counter", "abc", "reset").Send(restate.Void{}, 0); err != nil {
							return restate.Void{}, err
						}
					}

					// This is required to cause a suspension after the non-deterministic operation
					ctx.Sleep(100 * time.Millisecond)
					return restate.Void{}, incrementCounter(ctx)
				})).
			Handler("setDifferentKey", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					if doLeftAction(ctx) {
						if err := ctx.Set(STATE_A, "my-state"); err != nil {
							return restate.Void{}, err
						}
					} else {
						if err := ctx.Set(STATE_B, "my-state"); err != nil {
							return restate.Void{}, err
						}
					}

					// This is required to cause a suspension after the non-deterministic operation
					ctx.Sleep(100 * time.Millisecond)
					return restate.Void{}, incrementCounter(ctx)
				})))
}
