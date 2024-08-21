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
					restate.Set(ctx, value.Key, value.Value)
					return restate.Void{}, nil
				})).
			Handler("get", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, key string) (string, error) {
					return restate.Get[string](ctx, key)
				})).
			Handler("clearAll", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) ([]Entry, error) {
					keys, err := restate.Keys(ctx)
					if err != nil {
						return nil, err
					}
					out := make([]Entry, 0, len(keys))
					for _, k := range keys {
						value, err := restate.Get[string](ctx, k)
						if err != nil {
							return nil, err
						}
						out = append(out, Entry{Key: k, Value: value})
					}
					restate.ClearAll(ctx)
					return out, nil
				})))
}
