Ingress SDK Usage
=================

When calling a handler or workflow, or attaching to an invocation from outside of a restate context, 
the ingress API must be called via [http](https://docs.restate.dev/invoke/http). The following ingress 
functions provide an SDK for that purpose. These functions are analogous to the 
[facilitator](https://github.com/restatedev/sdk-go/blob/main/facilitators.go) functions 
that are used from within the restate context.

## Service

```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
output, err := ingress.Service[*MyInputStruct, *MyOutputStruct](
	client serviceName, handlerName).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## ServiceSend

```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
invocation := ingress.ServiceSend[*MyInputStruct](
	client, serviceName, handlerName).
	Send(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithDelay(time.Minute),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
if invocation.Error != nil {
	println(invocation.Error.Error())
}
println(invocation.Id)
```

## Object

```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
output, err := ingress.Object[*MyInputStruct, *MyOutputStruct](
	client, serviceName, objectKey, handlerName).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## ObjectSend

```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
invocation := ingress.ObjectSend[*MyInputStruct](
	client, serviceName, objectKey, handlerName).
	Send(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithDelay(time.Minute),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
if invocation.Error != nil {
	println(invocation.Error.Error())
}
println(invocation.Id)
```

## Workflow

```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
output, err := ingress.Workflow[*MyInputStruct, *MyOutputStruct](
	client, serviceName, workflowId, "send").
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## WorkflowSend

```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
invocation := ingress.WorkflowSend[*MyInputStruct](
	client, serviceName, workflowId, handlerName).
	Send(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithDelay(time.Minute),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
if invocation.Error != nil {
	println(invocation.Error.Error())
}
println(invocation.Id)
```

## AttachInvocation

**Blocking until invocation returns output**
```go
client := ingress.NewClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachInvocation[*MyOutputStruct](
	client, invocationId).
	Attach(ctx)
```

**Non-blocking, returns error if invocation is not found or not done**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachInvocation[*MyOutputStruct](
	client, invocationId).
	Output(ctx)
```

## AttachService

**Blocking until service invocation returns output**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachService[*MyOutputStruct](
	client, serviceName, handlerName, idempotencyKey).
	Attach(ctx)
```

**Non-blocking, returns error if service invocation is not found or not done**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachService[*MyOutputStruct](
	client, serviceName, handlerName, idempotencyKey).
	Output(ctx)
```

## AttachObject

**Blocking until object invocation returns output**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachObject[*MyOutputStruct](
	client, serviceName, objectKey, handlerName, idempotencyKey).
	Attach(ctx)
```

**Non-blocking, returns error if object invocation is not found or not done**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachObject[*MyOutputStruct](
	client, serviceName, objectKey, handlerName, idempotencyKey).
	Output(ctx)
```

## AttachWorkflow

**Blocking until workflow returns output**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachWorkflow[*MyOutputStruct](
	client, serviceName, workflowId).
	Attach(ctx)
```

**Non-blocking, returns error if workflow is not found or not done**
```go
client := ingress.NewClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := ingress.AttachWorkflow[*MyOutputStruct](
	client, serviceName, workflowId).
	Output(ctx)
```
