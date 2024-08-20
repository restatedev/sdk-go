package main

import (
	restate "github.com/restatedev/sdk-go"
)

func init() {
	REGISTRY.AddDefinition(restate.NewService("KillTestRunner").Handler("startCallTree", restate.NewServiceHandler(func(ctx restate.Context, _ restate.Void) (restate.Void, error) {
		return restate.Void{}, ctx.Object("KillTestSingleton", "", "recursiveCall").Request(restate.Void{}, restate.Void{})
	})))

	REGISTRY.AddDefinition(
		restate.NewObject("KillTestSingleton").
			Handler("recursiveCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					awakeable := ctx.Awakeable()
					ctx.Object("AwakeableHolder", "kill", "hold").Send(awakeable.Id())
					if err := awakeable.Result(restate.Void{}); err != nil {
						return restate.Void{}, err
					}

					return restate.CallAs[restate.Void](ctx.Object("KillTestSingleton", "", "recursiveCall")).Request(restate.Void{})
				})).
			Handler("isUnlocked", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					// no-op
					return restate.Void{}, nil
				})))
}
