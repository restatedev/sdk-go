package main

import (
	restate "github.com/restatedev/sdk-go"
)

func init() {
	REGISTRY.AddDefinition(restate.NewService("KillTestRunner").Handler("startCallTree", restate.NewServiceHandler(func(ctx restate.Context, _ restate.Void) (restate.Void, error) {
		return restate.Object[restate.Void](ctx, "KillTestSingleton", "", "recursiveCall").Request(restate.Void{})
	})))

	REGISTRY.AddDefinition(
		restate.NewObject("KillTestSingleton").
			Handler("recursiveCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					awakeable := restate.Awakeable[restate.Void](ctx)
					restate.ObjectSend(ctx, "AwakeableHolder", "kill", "hold").Send(awakeable.Id())
					if _, err := awakeable.Result(); err != nil {
						return restate.Void{}, err
					}

					return restate.Object[restate.Void](ctx, "KillTestSingleton", "", "recursiveCall").Request(restate.Void{})
				})).
			Handler("isUnlocked", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					// no-op
					return restate.Void{}, nil
				})))
}
