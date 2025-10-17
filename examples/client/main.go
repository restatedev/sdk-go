package main

import (
	"context"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	restateingress "github.com/restatedev/sdk-go/ingress"
)

// This file serves as a tutorial for the Restate Ingress SDK
// It demonstrates how to use the various functions provided by the SDK
// The code is meant to be illustrative and may not run as-is

func main() {
	// This main function demonstrates various ways to use the Restate Ingress SDK
	// Each section is commented to explain what it does

	// ==========================================
	// Ingress Functions
	// ==========================================

	// Service Example
	// This demonstrates how to call a service handler
	serviceExample()

	// ServiceSend Example
	// This demonstrates how to send a message to a service handler
	serviceSendExample()

	// Object Example
	// This demonstrates how to call an object handler
	objectExample()

	// ObjectSend Example
	// This demonstrates how to send a message to an object handler
	objectSendExample()

	// Workflow Example
	// This demonstrates how to call a workflow
	workflowExample()

	// WorkflowSend Example
	// This demonstrates how to send a message to a workflow
	workflowSendExample()

	// ==========================================
	// Invocation Attach Functions
	// ==========================================

	// AttachInvocation Example
	// This demonstrates how to attach to an invocation
	attachInvocationExample()

	// AttachService Example
	// This demonstrates how to attach to a service invocation
	attachServiceExample()

	// AttachObject Example
	// This demonstrates how to attach to an object invocation
	attachObjectExample()

	// AttachWorkflow Example
	// This demonstrates how to attach to a workflow
	attachWorkflowExample()

	fmt.Println("Tutorial completed!")
}

// ==========================================
// Ingress Functions Implementation
// ==========================================

func serviceExample() {
	// Service Example
	// When calling a handler from outside of a restate context

	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	var input MyInput
	input.Name = "World"

	output, err := restateingress.Service[*MyInput, *MyOutput](
		client, "ServiceName", "handlerName").
		Request(context.Background(), &input,
			restate.WithIdempotencyKey("idem-key-1"),
			restate.WithHeaders(map[string]string{"header-name": "header-value"}))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Service output: %v\n", output)
}

func serviceSendExample() {
	// ServiceSend Example
	// When sending a message to a handler without waiting for a response

	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	var input MyInput
	input.Name = "World"

	invocation, err := restateingress.ServiceSend[*MyInput](
		client, "ServiceName", "handlerName").
		Send(context.Background(), &input,
			restate.WithIdempotencyKey("idem-key-1"),
			restate.WithDelay(time.Minute),
			restate.WithHeaders(map[string]string{"header-name": "header-value"}))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("ServiceSend invocation ID:", invocation.Id())
}

func objectExample() {
	// Object Example
	// When calling an object handler from outside of a restate context

	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	var input MyInput
	input.Name = "World"

	output, err := restateingress.Object[*MyInput, *MyOutput](
		client, "ServiceName", "objectKey", "handlerName").
		Request(context.Background(), &input,
			restate.WithIdempotencyKey("idem-key-1"),
			restate.WithHeaders(map[string]string{"header-name": "header-value"}))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Object output: %v\n", output)
}

func objectSendExample() {
	// ObjectSend Example
	// When sending a message to an object handler without waiting for a response

	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	var input MyInput
	input.Name = "World"

	invocation, err := restateingress.ObjectSend[*MyInput](
		client, "ServiceName", "objectKey", "handlerName").
		Send(context.Background(), &input,
			restate.WithIdempotencyKey("idem-key-1"),
			restate.WithDelay(time.Minute),
			restate.WithHeaders(map[string]string{"header-name": "header-value"}))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("ObjectSend invocation ID:", invocation.Id())
}

func workflowExample() {
	// Workflow Example
	// When calling a workflow from outside of a restate context

	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	var input MyInput
	input.Name = "World"

	output, err := restateingress.Workflow[*MyInput, *MyOutput](
		client, "ServiceName", "workflowId", "send").
		Request(context.Background(), &input,
			restate.WithIdempotencyKey("idem-key-1"),
			restate.WithHeaders(map[string]string{"header-name": "header-value"}))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Workflow output: %v\n", output)
}

func workflowSendExample() {
	// WorkflowSend Example
	// When sending a message to a workflow without waiting for a response

	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	var input MyInput
	input.Name = "World"

	invocation, err := restateingress.WorkflowSend[*MyInput](
		client, "ServiceName", "workflowId", "handlerName").
		Send(context.Background(), &input,
			restate.WithIdempotencyKey("idem-key-1"),
			restate.WithDelay(time.Minute),
			restate.WithHeaders(map[string]string{"header-name": "header-value"}))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("WorkflowSend invocation ID:", invocation.Id())
}

// ==========================================
// Invocation Attachment Functions Implementation
// ==========================================

func attachInvocationExample() {
	// AttachInvocation Example
	// These functions return the output of the invocation

	// Blocking until invocation returns output
	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	invocationId := "some-invocation-id"

	output, err := restateingress.InvocationById[*MyOutput](
		client, invocationId).
		Attach(context.Background())

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("AttachInvocation output: %v\n", output)

	// Non-blocking, returns error if invocation is not complete
	outputNonBlocking, errNonBlocking := restateingress.InvocationById[*MyOutput](
		client, invocationId).
		Output(context.Background())

	if errNonBlocking != nil {
		fmt.Println("Non-blocking error:", errNonBlocking)
		return
	}

	fmt.Printf("AttachInvocation non-blocking output: %v\n", outputNonBlocking)
}

func attachServiceExample() {
	// AttachService Example
	// These functions return the output of the service invocation

	// Blocking until service invocation returns output
	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	serviceName := "ServiceName"
	handlerName := "handlerName"
	idempotencyKey := "idem-key-1"

	output, err := restateingress.AttachService[*MyOutput](
		client, serviceName, handlerName, idempotencyKey).
		Attach(context.Background())

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("AttachService output: %v\n", output)

	// Non-blocking, returns error if service invocation is not complete
	outputNonBlocking, errNonBlocking := restateingress.ServiceInvocationByIdempotencyKey[*MyOutput](
		client, serviceName, handlerName, idempotencyKey).
		Output(context.Background())

	if errNonBlocking != nil {
		fmt.Println("Non-blocking error:", errNonBlocking)
		return
	}

	fmt.Printf("AttachService non-blocking output: %v\n", outputNonBlocking)
}

func attachObjectExample() {
	// AttachObject Example
	// These functions return the output of the object invocation

	// Blocking until object invocation returns output
	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	serviceName := "ServiceName"
	objectKey := "objectKey"
	handlerName := "handlerName"
	idempotencyKey := "idem-key-1"

	output, err := restateingress.ObjectInvocationByIdempotencyKey[*MyOutput](
		client, serviceName, objectKey, handlerName, idempotencyKey).
		Attach(context.Background())

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("AttachObject output: %v\n", output)

	// Non-blocking, returns error if object invocation is not complete
	outputNonBlocking, errNonBlocking := restateingress.ObjectInvocationByIdempotencyKey[*MyOutput](
		client, serviceName, objectKey, handlerName, idempotencyKey).
		Output(context.Background())

	if errNonBlocking != nil {
		fmt.Println("Non-blocking error:", errNonBlocking)
		return
	}

	fmt.Printf("AttachObject non-blocking output: %v\n", outputNonBlocking)
}

func attachWorkflowExample() {
	// AttachWorkflow Example
	// These functions return the output of the workflow

	// Blocking until workflow returns output
	client := restateingress.NewClient("http://localhost:8080",
		restate.WithAuthKey("authkey"))

	serviceName := "ServiceName"
	workflowId := "workflowId"

	output, err := restateingress.WorkflowHandle[*MyOutput](
		client, serviceName, workflowId).
		Attach(context.Background())

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("AttachWorkflow output: %v\n", output)

	// Non-blocking, returns error if workflow is not complete
	outputNonBlocking, errNonBlocking := restateingress.WorkflowHandle[*MyOutput](
		client, serviceName, workflowId).
		Output(context.Background())

	if errNonBlocking != nil {
		fmt.Println("Non-blocking error:", errNonBlocking)
		return
	}

	fmt.Printf("AttachWorkflow non-blocking output: %v\n", outputNonBlocking)
}

// --- Mock data structures

type MyInput struct {
	Name string
}

type MyOutput struct {
	Greeting string
}
