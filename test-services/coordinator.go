package main

import (
	restate "github.com/restatedev/sdk-go"
)

func RegisterCoordinator() {
	REGISTRY.AddRouter(
		restate.NewServiceRouter("Coordinator").
			Handler("proxy", restate.NewServiceHandler(
				func(ctx restate.Context, _ restate.Void) (string, error) {
					key := ctx.Rand().UUID().String()
					return restate.CallAs[string](ctx.Object("Receiver", key, "ping")).Request(nil)
				})))
}

type coordinator struct {
}

func (s coordinator) ServiceName() string {
	return "Coordinator"
}

func (s coordinator) Proxy(ctx restate.Context, _ restate.Void) (string, error) {
	key := ctx.Rand().UUID().String()
	return restate.CallAs[string](ctx.Object("Receiver", key, "ping")).Request(nil)
}
