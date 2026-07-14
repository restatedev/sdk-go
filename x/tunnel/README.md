# restatedev/sdk-go/x/tunnel

Serve a Restate SDK deployment over an **outbound** connection to Restate Cloud's
tunnel servers — no inbound HTTP listener, no public ingress. This is the Go
equivalent of the TypeScript [`@restatedev/restate-sdk-tunnel`](https://github.com/restatedev/sdk-typescript/tree/main/packages/libs/restate-sdk-tunnel)
package, and it interoperates with the [restate-operator](https://github.com/restatedev/restate-operator)'s
in-process tunnel mode.

> **Experimental.** This is an `x/` module on a `0.x` line; its API may change
> between minor versions.

## Usage

Instead of `server.Restate.Start` (which listens for inbound connections), build a
`*server.Restate` with your services, wrap it with `tunnel.NewTunnel`, and call
`Start`. `Start` blocks until `ctx` is cancelled (or a fatal error stops the
tunnel), draining in-flight invocations before it returns — the tunnel equivalent
of `server.Restate.Start`:

```go
srv := server.NewRestate().
	Bind(restate.Reflect(Greeter{}))

err := tunnel.NewTunnel(srv,
	tunnel.WithRegion("us"),
	tunnel.WithEnvironment("env_...", "publickeyv1_..."),
	tunnel.WithAuthToken(os.Getenv("RESTATE_AUTH_TOKEN")),
	tunnel.WithTunnelName("greeter-v1"),
).Start(ctx)
```

`Start` logs the deployment URL once connected; register it (the operator does
this for you on Kubernetes):

```sh
restate deployments register <deployment-url>
```

If you need the URL programmatically (or want to manage the lifecycle yourself),
use `Connect` instead of `Start` for a non-blocking `*Connection` handle with
`Ready`, `DeploymentURL`, `ConnectionCount`, `Err`, `Shutdown`, and `Close`.

### Zero-config under the operator

With `tunnelMode: in-process`, the restate-operator injects the `RESTATE_INPROC_*`
environment variables. Any option left unset falls back to them, so
`tunnel.NewTunnel(srv).Start(ctx)` plus a mounted token file is a complete
configuration:

| Env var | Option |
|---|---|
| `RESTATE_INPROC_CLOUD_REGION` | `WithRegion` |
| `RESTATE_TUNNEL_SERVERS_SRV` | `WithServersSRV` |
| `RESTATE_INPROC_ENVIRONMENT_ID` | `WithEnvironment` (1st arg) |
| `RESTATE_INPROC_SIGNING_PUBLIC_KEY` | `WithEnvironment` (2nd arg) |
| `RESTATE_INPROC_TUNNEL_NAME` | `WithTunnelName` |
| `RESTATE_AUTH_TOKEN` | `WithAuthToken` |
| `RESTATE_INPROC_AUTH_TOKEN_FILE` | `WithAuthTokenFile` (re-read each reconnect) |
| `RESTATE_TUNNEL_WORKER_ID` | `WithWorkerID` (`HOSTNAME` seeds the default) |

`WithServers`, `WithTLS`, `WithLogger`, and the tuning options have no environment
fallback. Discovery (`WithRegion`/`WithServersSRV`/`WithServers`) env vars only
apply when no discovery source is set explicitly.

## How it works

The SDK dials out over TLS (ALPN `h2`), then **role-flips**: Restate Cloud drives
the connection as the HTTP/2 *client* while the SDK serves as the HTTP/2 *server*
(`http2.Server.ServeConn`). The tunnel server opens `GET /_/start-tunnel` and
completes a handshake via HTTP/2 trailers; afterwards each invocation is one
stream whose `/<scheme>/<host>/<port>` prefix is stripped before the request is
handed to the reused `server.Restate` handler (which performs discovery, invoke,
and request-identity verification against `SigningPublicKey`).

## Notes

- **Multi-homing.** With `WithRegion`/`WithServersSRV`, the SRV name is resolved
  to every A/AAAA address and one connection is kept per resolved IP; the set is
  re-resolved periodically (`WithResolveInterval`, default 30s) and reconciled as
  DNS changes. Each connection runs its own reconnect loop; a fatal handshake on
  any of them (shared credentials) stops the whole tunnel.
- **Liveness.** A server-initiated HTTP/2 PING watchdog detects half-open peers:
  after a connection is read-idle for the ping interval it sends a PING, and if it
  isn't acked within the ping timeout the connection is torn down and reconnected
  (`WithLivenessPing`, defaults 75s/10s).
- **Trailer handling.** The tunnel server sends the handshake result as
  *unannounced* HTTP/2 request trailers, which Go's `net/http`/`x/net/http2`
  server would otherwise drop. The tunnel transparently injects the required
  `Trailer` announcement into the handshake stream so they are surfaced. See
  `hframe.go`.
- **Not yet implemented:** zero-drop server-drain rollover (on `/_/drain-tunnel`
  the connection drains in-flight then closes and reconnects, rather than keeping
  the old connection serving while a replacement is dialed), and granular
  TLS/mTLS option fields (use `WithTLS(*tls.Config)`).
