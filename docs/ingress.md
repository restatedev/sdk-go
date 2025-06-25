Ingress SDK Usage
=================

When calling a handler or workflow outside of a restate context, the ingress service
must be invoked via [http](https://docs.restate.dev/invoke/http). The following ingress 
functions provide an SDK for that purpose. These functions are analogous to the 
[facilitator](https://github.com/restatedev/sdk-go/blob/main/facilitators.go) functions 
that are used from within the restate context.

## IngressService

```go
var input MyInputStruct
output, err := restate.IngressService[*MyInputStruct, *MyOutputStruct](
	serviceName, handlerName, restate.WithBaseUrl("http://localhost:8080")).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## IngressServiceSend

```go
var input MyInputStruct
invocation := restate.IngressServiceSend[*MyInputStruct](
	serviceName, handlerName, restate.WithBaseUrl("http://localhost:8080")).
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
var input MyInputStruct
output, err := restate.IngressObject[*MyInputStruct, *MyOutputStruct](
	serviceName, objectKey, handlerName, restate.WithBaseUrl("http://localhost:8080")).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## IngressObjectSend

```go
var input MyInputStruct
invocation := restate.IngressObjectSend[*MyInputStruct](
	serviceName, objectKey, handlerName, restate.WithBaseUrl("http://localhost:8080")).
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
var input MyInputStruct
output, err := restate.IngressWorkflow[*MyInputStruct, *MyOutputStruct](
	serviceName, workflowId, "send", restate.WithBaseUrl("http://localhost:8080")).
	Request(ctx, &input, 
		restate.WithIdempotencyKey("idem-key-1"),
		restate.WithHeaders(map[string]string{"header-name","header-value"}))
```

## IngressWorkflowSend

```go
var input MyInputStruct
invocation := restate.IngressWorkflowSend[*MyInputStruct](
	serviceName, workflowId, handlerName, restate.WithBaseUrl("http://localhost:8080")).
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
output, err := restate.IngressAttachInvocation[*MyOutputStruct](
	invocationId, restate.WithBaseUrl("http://localhost:8080")).
	Attach(ctx)
```

**Non-blocking, returns error if invocation is not found or not done**
```go
output, err := restate.IngressAttachInvocation[*MyOutputStruct](
	invocationId, restate.WithBaseUrl("http://localhost:8080")).
	Output(ctx)
```

**Cancel an invocation. NOTE: use the admin base URL here.**
```go
err := restate.IngressAttachInvocation[any](
	invocationId, restate.WithBaseUrl("http://localhost:9070")).
	Cancel(ctx, restate.WithCancelMode(mode))
```
Where `mode` is either `restate.CancelModelCancel`, `restate.CancelModeKill` or `restate.CancelModePurge`.

## IngressAttachService

**Blocking until service invocation returns output**
```go
output, err := restate.IngressAttachService[*MyOutputStruct](
	serviceName, handlerName, idempotencyKey, restate.WithBaseUrl("http://localhost:8080")).
	Attach(ctx)
```

**Non-blocking, returns error if service invocation is not found or not done**
```go
output, err := restate.IngressAttachService[*MyOutputStruct](
	serviceName, handlerName, idempotencyKey, restate.WithBaseUrl("http://localhost:8080")).
	Output(ctx)
```

## IngressAttachObject

**Blocking until object invocation returns output**
```go
output, err := restate.IngressAttachObject[*MyOutputStruct](
	serviceName, objectKey, handlerName, idempotencyKey, restate.WithBaseUrl("http://localhost:8080")).
	Attach(ctx)
```

**Non-blocking, returns error if object invocation is not found or not done**
```go
output, err := restate.IngressAttachObject[*MyOutputStruct](
	serviceName, objectKey, handlerName, idempotencyKey, restate.WithBaseUrl("http://localhost:8080")).
	Output(ctx)
```

## IngressAttachWorkflow

**Blocking until workflow returns output**
```go
output, err := restate.IngressAttachWorkflow[*MyOutputStruct](
	serviceName, workflowId, restate.WithBaseUrl("http://localhost:8080")).
	Attach(ctx)
```

**Non-blocking, returns error if workflow is not found or not done**
```go
output, err := restate.IngressAttachWorkflow[*MyOutputStruct](
	serviceName, workflowId, restate.WithBaseUrl("http://localhost:8080")).
	Output(ctx)
```
