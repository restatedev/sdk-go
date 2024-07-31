package main

import (
	restate "github.com/restatedev/sdk-go"
)

func RegisterReceiver() {
	REGISTRY.AddRouter(
		restate.NewObjectRouter("Receiver").
			Handler("ping", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, _ restate.Void) (string, error) {
					return "pong", nil
				})))
}
