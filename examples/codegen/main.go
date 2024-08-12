package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	restate "github.com/restatedev/sdk-go"
	helloworld "github.com/restatedev/sdk-go/examples/codegen/proto"
	"github.com/restatedev/sdk-go/server"
)

type greeter struct {
	helloworld.UnimplementedGreeterServer
}

func (greeter) SayHello(ctx restate.Context, req *helloworld.HelloRequest) (*helloworld.HelloResponse, error) {
	counter := helloworld.NewCounterClient(ctx, req.Name)
	count, err := counter.Add().
		Request(&helloworld.AddRequest{Delta: 1})
	if err != nil {
		return nil, err
	}
	return &helloworld.HelloResponse{
		Message: fmt.Sprintf("Hello, %s! Call number: %d", req.Name, count.Value),
	}, nil
}

type counter struct {
	helloworld.UnimplementedCounterServer
}

func (c counter) Add(ctx restate.ObjectContext, req *helloworld.AddRequest) (*helloworld.GetResponse, error) {
	count, err := restate.GetAs[int64](ctx, "counter")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return nil, err
	}

	watchers, err := restate.GetAs[[]string](ctx, "watchers")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return nil, err
	}

	count += req.Delta
	if err := ctx.Set("counter", count); err != nil {
		return nil, err
	}

	for _, awakeableID := range watchers {
		if err := ctx.ResolveAwakeable(awakeableID, count); err != nil {
			return nil, err
		}
	}
	ctx.Clear("watchers")

	return &helloworld.GetResponse{Value: count}, nil
}

func (c counter) Get(ctx restate.ObjectSharedContext, _ *helloworld.GetRequest) (*helloworld.GetResponse, error) {
	count, err := restate.GetAs[int64](ctx, "counter")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return nil, err
	}

	return &helloworld.GetResponse{Value: count}, nil
}

func (c counter) AddWatcher(ctx restate.ObjectContext, req *helloworld.AddWatcherRequest) (*helloworld.AddWatcherResponse, error) {
	watchers, err := restate.GetAs[[]string](ctx, "watchers")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return nil, err
	}
	watchers = append(watchers, req.AwakeableId)
	if err := ctx.Set("watchers", watchers); err != nil {
		return nil, err
	}
	return &helloworld.AddWatcherResponse{}, nil
}

func (c counter) Watch(ctx restate.ObjectSharedContext, req *helloworld.WatchRequest) (*helloworld.GetResponse, error) {
	awakeable := restate.AwakeableAs[int64](ctx)

	// since this is a shared handler, we need to use a separate exclusive handler to store the awakeable ID
	// if there is an in-flight Add call, this will take effect after it completes
	// we could add a version counter check here to detect changes that happen mid-request and return immediately
	if _, err := helloworld.NewCounterClient(ctx, ctx.Key()).
		AddWatcher().
		Request(&helloworld.AddWatcherRequest{AwakeableId: awakeable.Id()}); err != nil {
		return nil, err
	}

	timeout := time.Duration(req.TimeoutMillis) * time.Millisecond
	if timeout == 0 {
		// infinite timeout case; just await the next value
		next, err := awakeable.Result()
		if err != nil {
			return nil, err
		}

		return &helloworld.GetResponse{Value: next}, nil
	}

	after := ctx.After(timeout)

	// this is the safe way to race two results
	selector := ctx.Select(after, awakeable)

	if selector.Select() == after {
		// the timeout won
		if err := after.Done(); err != nil {
			// an error here implies this invocation was cancelled
			return nil, err
		}
		return nil, restate.TerminalError(context.DeadlineExceeded, 408)
	}

	// otherwise, the awakeable won
	next, err := awakeable.Result()
	if err != nil {
		return nil, err
	}
	return &helloworld.GetResponse{Value: next}, nil
}

func main() {
	server := server.NewRestate().
		Bind(helloworld.NewGreeterServer(greeter{})).
		Bind(helloworld.NewCounterServer(counter{}))

	if err := server.Start(context.Background(), ":9080"); err != nil {
		slog.Error("application exited unexpectedly", "err", err.Error())
		os.Exit(1)
	}
}
