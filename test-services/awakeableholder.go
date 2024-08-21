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
					ctx.Set(ID_KEY, id)
					return restate.Void{}, nil
				})).
			Handler("hasAwakeable", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (bool, error) {
					_, err := restate.GetAs[string](ctx, ID_KEY)
					if err != nil {
						return false, err
					}
					return err == nil, nil
				})).
			Handler("unlock", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, payload string) (restate.Void, error) {
					id, err := restate.GetAs[string](ctx, ID_KEY)
					if err != nil {
						return restate.Void{}, err
					}
					if id == "" {
						return restate.Void{}, restate.TerminalError(fmt.Errorf("No awakeable registered"), 404)
					}
					ctx.ResolveAwakeable(id, payload)
					return restate.Void{}, nil
				})))
}
