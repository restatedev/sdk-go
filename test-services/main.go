package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/restatedev/sdk-go/server"
)

func main() {
	// Accomodating for verification tests here
	logging := strings.ToLower(os.Getenv("RESTATE_LOGGING"))
	if logging == "error" {
		slog.SetLogLoggerLevel(slog.LevelError)
	} else if logging == "warn" {
		slog.SetLogLoggerLevel(slog.LevelWarn)
	}

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
