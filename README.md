[![Go Reference](https://pkg.go.dev/badge/github.com/restatedev/sdk-go.svg)](https://pkg.go.dev/github.com/restatedev/sdk-go)
[![Go](https://github.com/restatedev/sdk-go/actions/workflows/test.yaml/badge.svg)](https://github.com/restatedev/sdk-go/actions/workflows/test.yaml)

# Restate Go SDK

[Restate](https://restate.dev/) is a system for easily building resilient applications using *distributed durable async/await*. This repository contains the Restate SDK for writing services in **Golang**.

## Features implemented

- [x] Log replay (resume of execution on failure)
- [x] State (set/get/clear/clear-all/keys)
- [x] Remote service call over restate runtime
- [X] Delayed execution of remote services
- [X] Sleep
- [x] Run
- [x] Awakeable
- [x] Shared object handlers
- [ ] Workflows

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

Registration

```bash
restate deployments register --force -y http://localhost:9080
```

In yet a third terminal do the following steps

- Add tickets to basket

```bash
curl -v localhost:8080/UserSession/azmy/AddTicket \
    -H 'content-type: application/json' \
    -d '"ticket-1"'

# {"response":true}
curl -v localhost:8080/UserSession/azmy/AddTicket \
    -H 'content-type: application/json' \
    -d '"ticket-2"'
# {"response":true}
```

Trying adding the same tickets again should return `false` since they are already reserved. If you didn't check out the tickets in 15min (if you are inpatient like me change the delay in code to make it shorter)

Finally checkout

```bash
curl localhost:8080/UserSession/azmy/Checkout
#{"response":true}
```
