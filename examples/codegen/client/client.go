package main

import (
	"context"
	"fmt"

	"github.com/restatedev/sdk-go/client"
	helloworld "github.com/restatedev/sdk-go/examples/codegen/proto"
)

func main() {
	ctx, err := client.Connect(context.Background(), "http://127.0.0.1:8080")
	if err != nil {
		panic(err)
	}

	greeter := helloworld.NewGreeterIngressClient(ctx)
	greeting, err := greeter.SayHello().Request(&helloworld.HelloRequest{Name: "world"})
	if err != nil {
		panic(err)
	}
	fmt.Println(greeting.Message)

	workflow := helloworld.NewWorkflowIngressClient(ctx, "my-workflow")
	send, err := workflow.Run().Send(&helloworld.RunRequest{})
	if err != nil {
		panic(err)
	}
	status, err := workflow.Status().Request(&helloworld.StatusRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println("workflow running with invocation id", send.InvocationId, "and status", status.Status)

	if _, err := workflow.Finish().Request(&helloworld.FinishRequest{}); err != nil {
		panic(err)
	}
}
