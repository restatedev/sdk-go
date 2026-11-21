[![Go Reference](https://pkg.go.dev/badge/github.com/restatedev/sdk-go.svg)](https://pkg.go.dev/github.com/restatedev/sdk-go)
[![Go](https://github.com/restatedev/sdk-go/actions/workflows/test.yaml/badge.svg)](https://github.com/restatedev/sdk-go/actions/workflows/test.yaml)

# Restate Go SDK

[Restate](https://restate.dev/) is a system for easily building resilient applications using *distributed durable async/await*. This repository contains the Restate SDK for writing services in **Golang**.

## Community

* 🤗️ [Join our online community](https://discord.gg/skW3AZ6uGd) for help, sharing feedback and talking to the community.
* 📖 [Check out our documentation](https://docs.restate.dev) to get quickly started!
* 📣 [Follow us on Twitter](https://twitter.com/restatedev) for staying up to date.
* 🙋 [Create a GitHub issue](https://github.com/restatedev/sdk-java/issues) for requesting a new feature or reporting a problem.
* 🏠 [Visit our GitHub org](https://github.com/restatedev) for exploring other repositories.

## Prerequisites
- Go: >= 1.24.0

## Serving your services

Bind your services to a `server.Restate`, then serve them in one of two ways.

**Serve** listens for HTTP/2 connections on an address. Register it with Restate
(`restate deployments register http://localhost:9080`):

```go
err := server.NewRestate().
	Bind(restate.Reflect(Greeter{})).
	Serve(ctx, ":9080")
```

**Connect** opens an outbound connection to Restate Cloud instead, so the deployment
needs no inbound listener and no public ingress. Copy a connect key from the Restate
Cloud UI (Development → Configure SDK) into `RESTATE_CONNECT_KEY`, then:

```go
err := server.NewRestate().
	Bind(restate.Reflect(Greeter{})).
	Connect(ctx, "greeter")
```

Once connected, it logs how to register the deployment (`restate deployments register connect://greeter`).

Both block until `ctx` is cancelled. To plug the services into your own HTTP server, use
`Handler()`; for AWS Lambda, use `LambdaHandler()`.

## Examples

This repo contains an [example](examples) based on the [Ticket Reservation Service](https://github.com/restatedev/examples/tree/main/tutorials/tour-of-restate-go).

You can also check a list of examples available here: https://github.com/restatedev/examples?tab=readme-ov-file#go

### How to use the example

Download and run restate, as described here [v1.x](https://github.com/restatedev/restate/releases/)

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
restate deployments register http://localhost:9080
```

And do the following steps

- Add tickets to basket

```bash
curl -v localhost:8080/UserSession/azmy/AddTicket \
    -H 'content-type: application/json' \
    -d '"ticket-1"'

# true
curl -v localhost:8080/UserSession/azmy/AddTicket \
    -H 'content-type: application/json' \
    -d '"ticket-2"'
# true
```

Trying adding the same tickets again should return `false` since they are already reserved. If you didn't check out the tickets in 15min (if you are impatient change the delay in code to make it shorter)

- Check out

```bash
curl localhost:8080/UserSession/azmy/Checkout
# true
```

## Ingress SDK

When you need to call restate handlers or attach to invocations from outside the restate context,
use the [ingress SDK](examples/client/main.go).

For a Restate Cloud environment, copy an ingress key from the Cloud UI (Development → Configure Client)
into `RESTATE_INGRESS_KEY` and create the client with `ingress.NewCloudClient()`.

## Versions

This library follows [Semantic Versioning](https://semver.org/).

**Upgrading from 0.x?** See the [migration guide](MIGRATION.md).

Compatibility with Restate Server:

| Restate Server | sdk-go 1.0          |
|----------------|---------------------|
| < 1.3          | ❌                   |
| 1.3            | ✅ <sup>(1)(2)</sup> |
| 1.4            | ✅ <sup>(2)</sup>    |
| 1.5            | ✅                   |
| 1.6            | ✅                   |
| 1.7            | ✅                   |

<sup>(1)</sup> `WithAbortTimeout`, `WithEnableLazyState`, `WithIdempotencyRetention`, `WithInactivityTimeout`, `WithIngressPrivate`, `WithJournalRetention` and `WithWorkflowRetention` require Restate Server >= 1.4. Check the in-code documentation for more details.

<sup>(2)</sup> `WithInvocationRetryPolicy` requires Restate Server >= 1.5. Check the in-code documentation for more details.

Older `0.x` SDK releases are legacy and deprecated; see the [Restate 1.5 release notes](https://github.com/restatedev/restate/releases/tag/v1.5.0) for their compatibility and deprecation details.

## Contributing

We’re excited if you join the Restate community and start contributing!
Whether it is feature requests, bug reports, ideas & feedback or PRs, we appreciate any and all contributions.
We know that your time is precious and, therefore, deeply value any effort to contribute!
