[![Go Reference](https://pkg.go.dev/badge/github.com/muhamadazmy/restate-sdk-go.svg)](https://pkg.go.dev/github.com/muhamadazmy/restate-sdk-go)
[![Go](https://github.com/muhamadazmy/restate-sdk-go/actions/workflows/test.yaml/badge.svg)](https://github.com/muhamadazmy/restate-sdk-go/actions/workflows/test.yaml)

# Restate Go SDK

[Restate](https://restate.dev/) is a system for easily building resilient applications using *distributed durable async/await*. This repository contains the Restate SDK for writing services in **Golang**.

This SDK is an individual effort to build a golang SDK for restate runtime. The implementation is based on the service protocol documentation found [here](https://github.com/restatedev/service-protocol/blob/main/service-invocation-protocol.md) and a lot of experimentation with the protocol.

This means that it's not granted that this SDK matches exactly what `restate` has intended but it's a best effort interpretation of the docs

Since **service discovery** was not documented (or at least I could not find any documentation for it), the implementation is based on reverse engineering the TypeScript SDK.

This implementation of the SDK **ONLY** supports `dynrpc`. There is noway yet that you can define your service interface with `gRPC`

Calling other services right now is done completely by name, hence it's not very safe since you can miss up arguments list/type for example but hopefully later on we can generate stubs or use `gRPC` interfaces to define services.

## Features implemented

- [x] Log replay (resume of execution on failure)
- [x] State (set/get/clear/clear-all/keys)
- [x] Remote service call over restate runtime
- [X] Delayed execution of remote services
- [X] Sleep
- [x] Side effects
  - Implementation might differ from as intended by restate since it's not documented and based on reverse engineering of the TypeScript SDK
- [ ] Awakeable

## Basic usage

Please check [example](example) for a fully working example. The example tries to implement the same exact example provided by restate official docs and TypeScript SDK but with few changes. So I recommend you start there first before trying out this example.

### How to use the example

Download and run restate [v0.8](https://github.com/restatedev/restate/releases/tag/v0.8.0)

```bash
restate-server --wipe all
```

> Generally you don't have to use `--wipe all` but that is mainly for testing to make sure you starting from clean state

In another terminal run the example

```bash
cd restate-sdk-go/example
go run .
```

In yet a third terminal do the following steps

- Add tickets to basket

```bash
curl -v localhost:8080/UserSession/addTicket \
    -H 'content-type: application/json' \
    -d '{"key": "azmy", "request": "ticket-1"}'

# {"response":true}
curl -v localhost:8080/UserSession/addTicket \
    -H 'content-type: application/json' \
    -d '{"key": "azmy", "request": "ticket-2"}'
# {"response":true}
```

Trying adding the same tickets again should return `false` since they are already reserved. If you didn't check out the tickets in 15min (if you are inpatient like me change the delay in code to make it shorter)

Finally checkout

```bash
curl localhost:8080/UserSession/checkout \
    -H 'content-type: application/json' \
    -d '{"key": "azmy", "request": null}'
#{"response":true}
```
