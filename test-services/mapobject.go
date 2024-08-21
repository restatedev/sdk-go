package main

import (
	restate "github.com/restatedev/sdk-go"
)

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func init() {
	REGISTRY.AddDefinition(
		restate.NewObject("MapObject").
			Handler("set", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, value Entry) (restate.Void, error) {
					ctx.Set(value.Key, value.Value)
					return restate.Void{}, nil
				})).
			Handler("get", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, key string) (string, error) {
					return restate.GetAs[string](ctx, key)
				})).
			Handler("clearAll", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) ([]Entry, error) {
					keys, err := ctx.Keys()
					if err != nil {
						return nil, err
					}
					out := make([]Entry, 0, len(keys))
					for _, k := range keys {
						value, err := restate.GetAs[string](ctx, k)
						if err != nil {
							return nil, err
						}
						out = append(out, Entry{Key: k, Value: value})
					}
					ctx.ClearAll()
					return out, nil
				})))
}
