Ingress SDK Usage
=================

When calling a handler or workflow, or attaching to an invocation from outside of a restate context, 
the ingress API must be called via [http](https://docs.restate.dev/invoke/http). The following ingress 
functions provide an SDK for that purpose. These functions are analogous to the 
[facilitator](https://github.com/restatedev/sdk-go/blob/main/facilitators.go) functions 
that are used from within the restate context.

## IngressService

```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
output, err := restate.IngressService[*MyInputStruct, *MyOutputStruct](
	client serviceName, handlerName).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## IngressServiceSend

```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
invocation := restate.IngressServiceSend[*MyInputStruct](
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

## IngressObject

```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
output, err := restate.IngressObject[*MyInputStruct, *MyOutputStruct](
	client, serviceName, objectKey, handlerName).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## IngressObjectSend

```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
invocation := restate.IngressObjectSend[*MyInputStruct](
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

## IngressWorkflow

```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
output, err := restate.IngressWorkflow[*MyInputStruct, *MyOutputStruct](
	client, serviceName, workflowId, "send").
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## IngressWorkflowSend

```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
var input MyInputStruct
invocation := restate.IngressWorkflowSend[*MyInputStruct](
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

## IngressAttachInvocation

**Blocking until invocation returns output**
```go
client := restate.NewIngressClient("http://localhost:8080",
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachInvocation[*MyOutputStruct](
	client, invocationId).
	Attach(ctx)
```

**Non-blocking, returns error if invocation is not found or not done**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachInvocation[*MyOutputStruct](
	client, invocationId).
	Output(ctx)
```

## IngressAttachService

**Blocking until service invocation returns output**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachService[*MyOutputStruct](
	client, serviceName, handlerName, idempotencyKey).
	Attach(ctx)
```

**Non-blocking, returns error if service invocation is not found or not done**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachService[*MyOutputStruct](
	client, serviceName, handlerName, idempotencyKey).
	Output(ctx)
```

## IngressAttachObject

**Blocking until object invocation returns output**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachObject[*MyOutputStruct](
	client, serviceName, objectKey, handlerName, idempotencyKey).
	Attach(ctx)
```

**Non-blocking, returns error if object invocation is not found or not done**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachObject[*MyOutputStruct](
	client, serviceName, objectKey, handlerName, idempotencyKey).
	Output(ctx)
```

## IngressAttachWorkflow

**Blocking until workflow returns output**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachWorkflow[*MyOutputStruct](
	client, serviceName, workflowId).
	Attach(ctx)
```

**Non-blocking, returns error if workflow is not found or not done**
```go
client := restate.NewIngressClient("http://localhost:8080", 
	restate.WithAuthKey("authkey"))
output, err := restate.IngressAttachWorkflow[*MyOutputStruct](
	client, serviceName, workflowId).
	Output(ctx)
```
