package main

import (
	"context"
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
	count, err := restate.Get[int64](ctx, "counter")
	if err != nil {
		return nil, err
	}

	watchers, err := restate.Get[[]string](ctx, "watchers")
	if err != nil {
		return nil, err
	}

	count += req.Delta
	restate.Set(ctx, "counter", count)

	for _, awakeableID := range watchers {
		restate.ResolveAwakeable(ctx, awakeableID, count)
	}
	restate.Clear(ctx, "watchers")

	return &helloworld.GetResponse{Value: count}, nil
}

func (c counter) Get(ctx restate.ObjectSharedContext, _ *helloworld.GetRequest) (*helloworld.GetResponse, error) {
	count, err := restate.Get[int64](ctx, "counter")
	if err != nil {
		return nil, err
	}

	return &helloworld.GetResponse{Value: count}, nil
}

func (c counter) AddWatcher(ctx restate.ObjectContext, req *helloworld.AddWatcherRequest) (*helloworld.AddWatcherResponse, error) {
	watchers, err := restate.Get[[]string](ctx, "watchers")
	if err != nil {
		return nil, err
	}
	watchers = append(watchers, req.AwakeableId)
	restate.Set(ctx, "watchers", watchers)
	return &helloworld.AddWatcherResponse{}, nil
}

func (c counter) Watch(ctx restate.ObjectSharedContext, req *helloworld.WatchRequest) (*helloworld.GetResponse, error) {
	awakeable := restate.Awakeable[int64](ctx)

	// since this is a shared handler, we need to use a separate exclusive handler to store the awakeable ID
	// if there is an in-flight Add call, this will take effect after it completes
	// we could add a version counter check here to detect changes that happen mid-request and return immediately
	if _, err := helloworld.NewCounterClient(ctx, restate.Key(ctx)).
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

	after := restate.After(ctx, timeout)

	// this is the safe way to race two results
	resultFut, err := restate.WaitFirst(ctx, after)
	if err != nil {
		return nil, err
	}

	if resultFut == after {
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

type workflow struct {
	helloworld.UnimplementedWorkflowServer
}

func (workflow) Run(ctx restate.WorkflowContext, _ *helloworld.RunRequest) (*helloworld.RunResponse, error) {
	restate.Set(ctx, "status", "waiting")
	_, err := restate.Promise[restate.Void](ctx, "promise").Result()
	if err != nil {
		return nil, err
	}
	restate.Set(ctx, "status", "finished")
	return &helloworld.RunResponse{Status: "finished"}, nil
}

func (workflow) Finish(ctx restate.WorkflowSharedContext, _ *helloworld.FinishRequest) (*helloworld.FinishResponse, error) {
	return nil, restate.Promise[restate.Void](ctx, "promise").Resolve(restate.Void{})
}

func (workflow) Status(ctx restate.WorkflowSharedContext, _ *helloworld.StatusRequest) (*helloworld.StatusResponse, error) {
	status, err := restate.Get[string](ctx, "status")
	if err != nil {
		return nil, err
	}
	return &helloworld.StatusResponse{Status: status}, nil
}

func main() {
	server := server.NewRestate().
		Bind(helloworld.NewGreeterServer(greeter{})).
		Bind(helloworld.NewCounterServer(counter{})).
		Bind(helloworld.NewWorkflowServer(workflow{}))

	if err := server.Start(context.Background(), ":9080"); err != nil {
		slog.Error("application exited unexpectedly", "err", err.Error())
		os.Exit(1)
	}
}
