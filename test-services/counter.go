package main

import (
	"errors"
	"fmt"
	restate "github.com/restatedev/sdk-go"
)

const COUNTER_KEY = "counter"

type CounterUpdateResponse struct {
	OldValue int64 `json:"oldValue"`
	NewValue int64 `json:"newValue"`
}

func RegisterCounter() {
	REGISTRY.AddRouter(
		restate.NewObjectRouter("Counter").
			Handler("reset", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					ctx.Clear(COUNTER_KEY)
					return restate.Void{}, nil
				})).
			Handler("get", restate.NewObjectSharedHandler(
				func(ctx restate.ObjectSharedContext, _ restate.Void) (int64, error) {
					c, err := restate.GetAs[int64](ctx, COUNTER_KEY)
					if errors.Is(err, restate.ErrKeyNotFound) {
						c = 0
					} else if err != nil {
						return 0, err
					}
					return c, nil
				})).
			Handler("add", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, addend int64) (CounterUpdateResponse, error) {
					oldValue, err := restate.GetAs[int64](ctx, COUNTER_KEY)
					if errors.Is(err, restate.ErrKeyNotFound) {
						oldValue = 0
					} else if err != nil {
						return CounterUpdateResponse{}, err
					}

					newValue := oldValue + addend
					err = ctx.Set(COUNTER_KEY, newValue)

					return CounterUpdateResponse{
						OldValue: oldValue,
						NewValue: newValue,
					}, err
				})).
			Handler("addThenFail", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, addend int64) (restate.Void, error) {
					oldValue, err := restate.GetAs[int64](ctx, COUNTER_KEY)
					if errors.Is(err, restate.ErrKeyNotFound) {
						oldValue = 0
					} else if err != nil {
						return restate.Void{}, err
					}

					newValue := oldValue + addend
					err = ctx.Set(COUNTER_KEY, newValue)
					if err != nil {
						return restate.Void{}, err
					}

					return restate.Void{}, restate.TerminalError(fmt.Errorf("%s", ctx.Key()))
				})))
}
