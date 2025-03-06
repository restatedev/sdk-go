package main

import (
	restate "github.com/restatedev/sdk-go"
)

func init() {
	REGISTRY.AddDefinition(restate.NewObject("KillTestRunner").Handler("startCallTree", restate.NewObjectHandler(func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
		return restate.Object[restate.Void](ctx, "KillTestSingleton", restate.Key(ctx), "recursiveCall").Request(restate.Void{})
	})))

	REGISTRY.AddDefinition(
		restate.NewObject("KillTestSingleton").
			Handler("recursiveCall", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					awakeable := restate.Awakeable[restate.Void](ctx)
					restate.ObjectSend(ctx, "AwakeableHolder", restate.Key(ctx), "hold").Send(awakeable.Id())
					if _, err := awakeable.Result(); err != nil {
						return restate.Void{}, err
					}

					return restate.Object[restate.Void](ctx, "KillTestSingleton", restate.Key(ctx), "recursiveCall").Request(restate.Void{})
				})).
			Handler("isUnlocked", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (restate.Void, error) {
					// no-op
					return restate.Void{}, nil
				})))
}
