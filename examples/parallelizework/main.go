package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"strings"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
)

type fanOutWorker struct{}

func (c *fanOutWorker) ServiceName() string {
	return FanOutWorkerServiceName
}

const FanOutWorkerServiceName = "FanOutWorker"

func (c *fanOutWorker) Run(ctx restate.Context, commaSeparatedTasks string) (aggregatedResults string, err error) {
	tasks := strings.Split(commaSeparatedTasks, ",")

	// Run tasks in parallel
	var futs []restate.Selectable
	for _, task := range tasks {
		futs = append(futs, restate.RunAsync[string](ctx, func(ctx restate.RunContext) (string, error) {
			log.Printf("Heavy task %s running", task)
			if rand.Intn(2) == 1 {
				log.Printf("Heavy task %s failed", task)
				panic(fmt.Errorf("failed to complete heavy task %s", task))
			}
			log.Printf("Heavy task %s done", task)
			return task, nil
		}))
	}

	// Aggregate
	var results []string
	for fu, err := range restate.Wait(ctx, futs...) {
		if err != nil {
			return "", err
		}
		result, err := fu.(restate.RunAsyncFuture[string]).Result()
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}

	return strings.Join(results, "-"), nil
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	server := server.NewRestate().Bind(restate.Reflect(&fanOutWorker{}))

	if err := server.Start(context.Background(), ":9080"); err != nil {
		slog.Error("application exited unexpectedly", "err", err.Error())
		os.Exit(1)
	}
}
