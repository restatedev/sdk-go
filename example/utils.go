package main

import (
	"fmt"
	"math/big"

	restate "github.com/restatedev/sdk-go"
)

var health = restate.
	NewServiceRouter("health").
	Handler("ping", restate.NewServiceHandler(
		func(restate.Context, struct{}) (restate.Void, error) {
			return restate.Void{}, nil
		}))

var bigCounter = restate.
	NewObjectRouter("bigCounter").
	Handler("add", restate.NewObjectHandler(
		func(ctx restate.ObjectContext, deltaText string) (string, error) {
			delta, ok := big.NewInt(0).SetString(deltaText, 10)
			if !ok {
				return "", restate.TerminalError(fmt.Errorf("input must be a valid integer string: %s", deltaText))
			}

			bytes, err := restate.GetAs[[]byte](ctx, "counter", restate.WithBinary)
			if err != nil && err != restate.ErrKeyNotFound {
				return "", err
			}
			newCount := big.NewInt(0).Add(big.NewInt(0).SetBytes(bytes), delta)
			if err := ctx.Set("counter", newCount.Bytes(), restate.WithBinary); err != nil {
				return "", err
			}

			return newCount.String(), nil
		})).
	Handler("get", restate.NewObjectSharedHandler(
		func(ctx restate.ObjectSharedContext, input restate.Void) (string, error) {
			bytes, err := restate.GetAs[[]byte](ctx, "counter", restate.WithBinary)
			if err != nil {
				return "", err
			}

			return big.NewInt(0).SetBytes(bytes).String(), err
		}))
