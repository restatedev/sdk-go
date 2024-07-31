package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/restatedev/sdk-go/server"
)

func init() {
	// TODO register services
}

func main() {
	services := "*"
	if os.Getenv("SERVICES") != "" {
		services = os.Getenv("SERVICES")
	}

	server := server.NewRestate()

	if services == "*" {
		REGISTRY.RegisterAll(server)
	} else {
		fqdns := strings.Split(services, ",")
		set := make(map[string]struct{}, len(fqdns))
		for _, fqdn := range fqdns {
			set[fqdn] = struct{}{}
		}
		REGISTRY.Register(set, server)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9080"
	}

	if err := server.Start(context.Background(), ":"+port); err != nil {
		slog.Error("application exited unexpectedly", "err", err)
		os.Exit(1)
	}
}
