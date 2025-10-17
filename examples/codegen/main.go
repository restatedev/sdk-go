package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	restate "github.com/restatedev/sdk-go"
	helloworld "github.com/restatedev/sdk-go/examples/codegen/proto"
	"github.com/restatedev/sdk-go/ingress"
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

	go func() {
		time.Sleep(15 * time.Second)

		// Example usage of the generated ingress client

		c := ingress.NewClient("http://localhost:8080")

		counterClient := helloworld.NewCounterIngressClient(c, "fra")

		res, err := counterClient.Add().Send(context.Background(), &helloworld.AddRequest{Delta: 1}, restate.WithDelay(10*time.Second))
		if err != nil {
			slog.Error("failed to send request", "err", err.Error())
			os.Exit(1)
		}
		out, err := res.Attach(context.Background())
		if err != nil {
			slog.Error("failed to attach response", "err", err.Error())
			os.Exit(1)
		}
		slog.Info("client attached response", "out.value", out.Value)

		out, err = counterClient.Get().Request(context.Background(), &helloworld.GetRequest{})
		if err != nil {
			slog.Error("failed to get response", "err", err.Error())
			os.Exit(1)
		}
		slog.Info("client get response", "out.value", out.Value)
	}()

	if err := server.Start(context.Background(), ":9080"); err != nil {
		slog.Error("application exited unexpectedly", "err", err.Error())
		os.Exit(1)
	}
}
