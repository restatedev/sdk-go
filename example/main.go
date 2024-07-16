package main

import (
	"context"
	"log/slog"
	"os"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
)

func main() {

	server := server.NewRestate().
		// Handlers can be inferred from object methods
		Bind(restate.Object(&userSession{})).
		Bind(restate.Object(&ticketService{})).
		Bind(restate.Service(&checkout{})).
		// Or registered explicitly
		Bind(restate.NewServiceRouter("health").Handler("ping", restate.NewServiceHandler(
			func(restate.Context, struct{}) (restate.Void, error) {
				return restate.Void{}, nil
			}))).
		Bind(restate.NewObjectRouter("counter").Handler("add", restate.NewObjectHandler(
			func(ctx restate.ObjectContext, delta int) (int, error) {
				count, err := restate.GetAs[int](ctx, "counter")
				if err != nil && err != restate.ErrKeyNotFound {
					return 0, err
				}
				count += delta
				if err := restate.SetAs(ctx, "counter", count); err != nil {
					return 0, err
				}

				return count, nil
			})).Handler("get", restate.NewObjectSharedHandler(
			func(ctx restate.ObjectSharedContext, input restate.Void) (int, error) {
				return restate.GetAs[int](ctx, "counter")
			})))

	if err := server.Start(context.Background(), ":9080"); err != nil {
		slog.Error("application exited unexpectedly", "err", err.Error())
		os.Exit(1)
	}
}
