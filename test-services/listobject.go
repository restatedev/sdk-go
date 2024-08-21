package main

import (
	restate "github.com/restatedev/sdk-go"
)

const LIST_KEY = "list"

func init() {
	REGISTRY.AddDefinition(
		restate.NewObject("ListObject").
			Handler("append", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, value string) (restate.Void, error) {
					list, err := restate.Get[[]string](ctx, LIST_KEY)
					if err != nil {
						return restate.Void{}, err
					}
					list = append(list, value)
					restate.Set(ctx, LIST_KEY, list)
					return restate.Void{}, nil
				})).
			Handler("get", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) ([]string, error) {
					list, err := restate.Get[[]string](ctx, LIST_KEY)
					if err != nil {
						return nil, err
					}
					if list == nil {
						// or go would encode this as JSON null
						list = []string{}
					}

					return list, nil
				})).
			Handler("clear", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) ([]string, error) {
					list, err := restate.Get[[]string](ctx, LIST_KEY)
					if err != nil {
						return nil, err
					}
					if list == nil {
						// or go would encode this as JSON null
						list = []string{}
					}
					restate.Clear(ctx, LIST_KEY)
					return list, nil
				})))
}
