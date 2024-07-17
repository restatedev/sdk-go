[![Go Reference](https://pkg.go.dev/badge/github.com/restatedev/sdk-go.svg)](https://pkg.go.dev/github.com/restatedev/sdk-go)
[![Go](https://github.com/restatedev/sdk-go/actions/workflows/test.yaml/badge.svg)](https://github.com/restatedev/sdk-go/actions/workflows/test.yaml)

# Restate Go SDK

[Restate](https://restate.dev/) is a system for easily building resilient applications using *distributed durable async/await*. This repository contains the Restate SDK for writing services in **Golang**. This SDK has not yet seen
a stable release and APIs are subject to change. Feedback is welcome via
[issues](https://github.com/restatedev/sdk-go/issues/new) and in
[Discord](https://discord.gg/skW3AZ6uGd).

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

Please check [example](example) for a fully working example based on the
[TypeScript ticket reservation example]
(https://github.com/restatedev/examples/tree/main/patterns-use-cases/ticket-reservation/ticket-reservation-typescript)

### How to use the example

Download and run restate
[v1.x](https://github.com/restatedev/restate/releases/)

```bash
restate-server
```

In another terminal run the example

```bash
cd restate-sdk-go/example
go run .
```

In a third terminal register:

```bash
restate deployments register --force -y http://localhost:9080
```

And do the following steps

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

Trying adding the same tickets again should return `false` since they are already reserved. If you didn't check out the tickets in 15min (if you are impatient change the delay in code to make it shorter)

- Check out

```bash
curl localhost:8080/UserSession/azmy/Checkout
#{"response":true}
```

## Versions

This library follows [Semantic Versioning](https://semver.org/).

The compatibility with Restate is described in the following table:

| Restate Server\sdk-go | 0.9 |
|-------------------------|-----|
| 1.0                     | ✅   |

## Contributing

We’re excited if you join the Restate community and start contributing!
Whether it is feature requests, bug reports, ideas & feedback or PRs, we appreciate any and all contributions.
We know that your time is precious and, therefore, deeply value any effort to contribute!
