package main

import (
	"context"
	"fmt"
	restateingress "github.com/restatedev/sdk-go/ingress"
)

func main() {
	client := restateingress.NewClient("http://localhost:8080")

	output, err := restateingress.Service[string, string](
		client, "Greeter", "Greet").
		Request(context.Background(), "Francesco")

	if err != nil {
		panic(err)
	}

	fmt.Printf("Output: %v\n", output)
}
