# Migration guide: v0.24.0 → 1.0

## Reference of breaking changes

Apply this renames as search-and-replace. The sections below explain the why.

**Errors**
- `restate.TerminalError(err)` → `restate.ToTerminalError(err)`
- `restate.TerminalError(err, code)` → `restate.ToTerminalError(err, restate.WithErrorCode(code))`
- `restate.WithErrorCode(err, code)` → `restate.ToTerminalError(err, restate.WithErrorCode(code))` if terminal, otherwise `restate.ToRetryableError(err, restate.WithErrorCode(code))`
- `restate.ErrorCode(err)` → `restate.AsTerminalError(err).Code()` if terminal, or `restate.AsRetryableError(err).Code()` (nil-check the result first)
- `Get`/`Keys`/`Sleep`/`Wait`/`WaitFirst` now return `restate.TerminalError` (still an `error`)

**Request headers**
- `ctx.Request().Headers[k]` → `ctx.Request().Headers.Get(k)`
- `range ctx.Request().Headers` → `range ctx.Request().Headers.Iter()`
- need a plain map → `ctx.Request().Headers.ToMap()`

**Randomness**
- `restate.Rand(ctx).UUID()` → `restate.UUID(ctx)`
- `restate.Rand(ctx)` is now `*math/rand/v2.Rand` (use its methods)

**Retry options** (invocation-policy builders only; `Run` names unchanged)
- `WithMaxAttempts(int)` → `WithMaxRetryAttempts(uint)`
- `WithInitialInterval` → `WithInitialRetryInterval`
- `WithMaxInterval` → `WithMaxRetryInterval`
- `WithExponentiationFactor(float64)` → `WithRetryIntervalFactor(float32)`

**Codecs**
- `restate.WithPayloadCodec(c)` → `restate.WithCodec(c)`
- `encoding.PayloadCodec` → `encoding.Codec`
- custom codec with `InputPayload`/`OutputPayload` methods → implement `encoding.CodecMetadata` (+ `CodecInputMetadata`/`CodecOutputMetadata`) instead

**Ingress** (moved from `restate` to the `ingress` package)
- `restate.WithHttpClient` → `ingress.WithHttpClient`
- `restate.WithAuthKey` → `ingress.WithAuthKey`
- `restate.IngressClientOption` → `ingress.ClientOption`
- `restate.IngressRequestOption` → `ingress.RequestOption`
- `restate.IngressSendOption` → `ingress.SendOption`

**Import paths** (each now its own module — `go get`/`go install`)
- `github.com/restatedev/sdk-go/mocks` → `github.com/restatedev/sdk-go/x/mocks`
- `github.com/restatedev/sdk-go/rcontext` → `github.com/restatedev/sdk-go/logging`
- `github.com/restatedev/sdk-go/protoc-gen-go-restate` → `github.com/restatedev/sdk-go/x/protoc-gen-go-restate` (regenerate; contract import path → `…/x/protoc-gen-go-restate/generated/dev/restate/sdk`)
- `github.com/restatedev/sdk-go/testing` — same path, now a separate module (`go get` it)

## Errors

Failures are now **explicit types** instead of opaque `error`s, each carrying the extra
fields it supports: [`TerminalError`] completes the invocation with a failure (a status
code and optional metadata), and [`RetryableError`] is a non-terminal failure that is
retried (a status code). 

Because `TerminalError` is now a *type*, the v0.24.0 `TerminalError(...)` *constructor
function* had to be renamed to **`ToTerminalError`** (it takes the same `error`):

> ```go
> restate.TerminalError(err)        // → restate.ToTerminalError(err)
> restate.TerminalError(err, 409)   // → restate.ToTerminalError(err, restate.WithErrorCode(409))
> ```

| v0.24.0                       | 1.0                                                                                           |
|-------------------------------|-----------------------------------------------------------------------------------------------|
| `TerminalError(err)` *(func)* | `ToTerminalError(err)`                                                                        |
| `TerminalError(err, 409)`     | `ToTerminalError(err, WithErrorCode(409))`                                                    |
| `WithErrorCode(err, 409)`     | `ToTerminalError(err, WithErrorCode(409))` or `ToRetryableError(err, WithErrorCode(409))`     |
| `TerminalErrorf("…")`         | `TerminalErrorf("…")` *(unchanged; now returns `TerminalError`)*                              |
| `IsTerminalError(err)`        | `IsTerminalError(err)` *(unchanged)*                                                          |
| `ErrorCode(err)`              | removed — use `AsTerminalError(err)` or `AsRetryableError(err)` for downcasting to error type |

Operations that can only fail terminally now return [`TerminalError`] directly instead of
a plain `error` — `Get`, `Keys`, `Sleep`, `Wait`, `WaitFirst` — to make that explicit in
the signature. `TerminalError` still satisfies `error`, so most call sites are unaffected.

[`TerminalError`]: https://pkg.go.dev/github.com/restatedev/sdk-go#TerminalError
[`RetryableError`]: https://pkg.go.dev/github.com/restatedev/sdk-go#RetryableError

## Request headers

`ctx.Request().Headers` now returns **`restate.StringMap`** instead of `map[string]string`:

```go
// v0.24.0
h := ctx.Request().Headers["traceparent"]
for k, v := range ctx.Request().Headers { /* ... */ }
// 1.0
h := ctx.Request().Headers.Get("traceparent")
for k, v := range ctx.Request().Headers.Iter() { /* iterate safely over it */ }
m := ctx.Request().Headers.ToMap()   // when you need a plain map
```

## Randomness

`Rand(ctx)` now returns a standard-library `*math/rand/v2.Rand` instead of the custom
interface, so the full stdlib API is available. It is still seeded deterministically (same
sequence on every replay) and must not be used inside `Run` blocks. For UUIDs use
`restate.UUID(ctx)`.

```go
// v0.24.0
id := restate.Rand(ctx).UUID().String()
// 1.0
id := restate.UUID(ctx).String()

// the whole math/rand/v2 surface now works (deterministic, replay-safe):
r := restate.Rand(ctx)
roll := r.IntN(6) + 1
r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
```

In tests (`x/mocks`), the old `MockRand()` interface mock is gone. Use either:
```go
mockCtx.EXPECT().RandUUID().Return(uuid.Max)   // force a specific UUID
mockCtx.EXPECT().WithRandSeed(42)              // deterministic Rand/UUID/RandSource from a seed
```

## Retry options

`Run` retry options and the invocation retry policy (`WithInvocationRetryPolicy`) now
share one vocabulary: the same builders work in both places (pass it to
`Run`/`RunAsync`/`RunVoid`, or to `WithInvocationRetryPolicy`). 

The **invocation-policy** builders were renamed to match:

| v0.24.0 (invocation policy)         | 1.0                                  |
|-------------------------------------|--------------------------------------|
| `WithMaxAttempts(int)`              | `WithMaxRetryAttempts(uint)`         |
| `WithInitialInterval`               | `WithInitialRetryInterval`           |
| `WithMaxInterval`                   | `WithMaxRetryInterval`               |
| `WithExponentiationFactor(float64)` | `WithRetryIntervalFactor(float32)`   |

The `Run` option names are **unchanged**.

## Ingress client options

The ingress client options moved from `restate` to the `ingress` package:

| v0.24.0                        | 1.0                       |
|--------------------------------|---------------------------|
| `restate.WithHttpClient`       | `ingress.WithHttpClient`  |
| `restate.WithAuthKey`          | `ingress.WithAuthKey`     |
| `restate.IngressClientOption`  | `ingress.ClientOption`    |
| `restate.IngressRequestOption` | `ingress.RequestOption`   |
| `restate.IngressSendOption`    | `ingress.SendOption`      |

The options you pass to ingress requests/sends themselves are unchanged.

## Codecs

`PayloadCodec` and `WithPayloadCodec` is gone. Only `encoding.Codec` exists now and can be used everywhere, for handlers, services, ingress and value operations alike.
`WithProto`/`WithProtoJSON`/`WithBinary`/`WithJSON` are unchanged.

Handlers, in-process calls, and ingress requests can now set the input and output codec
**independently**:
```go
restate.NewServiceHandler(fn, restate.WithInputCodec(c1), restate.WithOutputCodec(c2))
restate.Service[O](ctx, "svc", "method", restate.WithInputCodec(c1), restate.WithOutputCodec(c2))
greeter.SayHello().Request(ctx, in, restate.WithInputCodec(c1), restate.WithOutputCodec(c2)) // ingress
```

`encoding.Codec` interface was split: now it contains only `Marshal`/`Unmarshal`, and the additional optional interfaces `encoding.CodecMetadata`, `encoding.CodecInputMetadata` and `encoding.CodecOutputMetadata` can be implemented to augment the discovery metadata.

Built-in codecs and the emitted manifest are unchanged.

## Removed deprecations

Symbols deprecated in v0.24.0 are gone. If you built against v0.24.0 with no deprecation
warnings, nothing to do; otherwise follow the replacement each notice already pointed to.

## Moved import paths

Only relevant if you import one of these. The packages are unchanged; just the path moved,
and each is now its own module you must `go get` explicitly.

- **Mocks** — `…/mocks` → `…/x/mocks`. `go get github.com/restatedev/sdk-go/x/mocks@latest`. Package still `mocks`.
- **Integration test env** — `…/testing` is now its own module. `go get github.com/restatedev/sdk-go/testing@latest`.
- **Custom `slog.Handler`** (replay status) — `…/rcontext` → `…/logging`. Same symbols (`LogContextFrom`, …), new path/package name.
- **Protoc plugin** — `…/protoc-gen-go-restate` → `…/x/protoc-gen-go-restate`. `go install …/x/protoc-gen-go-restate@latest` (binary name unchanged). Then **regenerate**: the contract's import path moved to `…/x/protoc-gen-go-restate/generated/dev/restate/sdk`. `buf` users: `buf dep update` (still published as `buf.build/restatedev/sdk-go`).
