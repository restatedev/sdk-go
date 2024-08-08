package main

import (
	"fmt"

	restate "github.com/restatedev/sdk-go"
)

const ID_KEY = "id"

func init() {
	REGISTRY.AddDefinition(
		restate.NewObject("AwakeableHolder").
			Handler("hold", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, id string) (restate.Void, error) {
					if err := ctx.Set(ID_KEY, id); err != nil {
						return restate.Void{}, err
					}
					return restate.Void{}, nil
				})).
			Handler("hasAwakeable", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
					_, err := restate.GetAs[string](ctx, ID_KEY)
					if err != nil && err != restate.ErrKeyNotFound {
						return false, err
					}
					return err == nil, nil
				})).
			Handler("unlock", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, payload string) (restate.Void, error) {
					id, err := restate.GetAs[string](ctx, ID_KEY)
					if err != nil {
						if err == restate.ErrKeyNotFound {
							return restate.Void{}, restate.TerminalError(fmt.Errorf("No awakeable registered"), 404)
						}
						return restate.Void{}, err
					}
					if err := ctx.ResolveAwakeable(id, payload); err != nil {
						return restate.Void{}, err
					}
					return restate.Void{}, nil
				})))
}
