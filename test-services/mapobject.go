package main

import (
	restate "github.com/restatedev/sdk-go"
)

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func init() {
	REGISTRY.AddRouter(
		restate.NewObjectRouter("MapObject").
			Handler("set", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, value Entry) (restate.Void, error) {
					return restate.Void{}, ctx.Set(value.Key, value.Value)
				})).
			Handler("get", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, key string) (string, error) {
					value, err := restate.GetAs[string](ctx, key)
					if err != nil && err != restate.ErrKeyNotFound {
						return "", err
					}
					return value, nil
				})).
			Handler("clearAll", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) ([]Entry, error) {
					keys := ctx.Keys()
					out := make([]Entry, 0, len(keys))
					for _, k := range keys {
						value, err := restate.GetAs[string](ctx, k)
						if err != nil {
							return nil, err
						}
						out = append(out, Entry{Key: k, Value: value})
					}
					return out, nil
				})))
}
