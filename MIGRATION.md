# Migration guide: v0.24.0 → 1.0

## Errors

`TerminalError` is now a **type** (a sealed interface with `Code()`, `Message()`,
`Metadata()`), not a constructor function. The constructors are `ToTerminalError` (from
an `error`) and `TerminalErrorf` (from a format string); the code and metadata are set
with the `WithErrorCode` and `WithMetadata` / `WithMetadataMap` **options**.

> **The rename you'll do most:** the v0.24.0 `TerminalError(...)` *function* is gone — the
> identifier is now the type. Replace calls with **`ToTerminalError`**, which takes the
> same `error` input:
>
> ```go
> restate.TerminalError(err)        // → restate.ToTerminalError(err)
> restate.TerminalError(err, 409)   // → restate.ToTerminalError(err, restate.WithErrorCode(409))
> ```

| v0.24.0                       | 1.0                                                                               |
|-------------------------------|-----------------------------------------------------------------------------------|
| `TerminalError(err)` *(func)* | `ToTerminalError(err)`                                                            |
| `TerminalError(err, 409)`     | `ToTerminalError(err, WithErrorCode(409))`                                        |
| `TerminalErrorf("…")`         | `TerminalErrorf("…")` *(unchanged; now returns `TerminalError`)*                  |
| `IsTerminalError(err)`        | `IsTerminalError(err)` *(unchanged)*                                              |
| `ErrorCode(err)`              | removed — use `if te := AsTerminalError(err); te != nil { te.Code() }`            |
| —                             | `AsTerminalError(err) TerminalError` *(new: typed accessor, nil if not terminal)* |

```go
// v0.24.0:  restate.TerminalError(fmt.Errorf("bad input: %w", err), http.StatusBadRequest)
// 1.0:
return restate.ToTerminalError(fmt.Errorf("bad input: %w", err), restate.WithErrorCode(http.StatusBadRequest))

// plain message, no code:
return restate.TerminalErrorf("bad input: %v", err)

// message + metadata:
return restate.ToTerminalError(fmt.Errorf("nope"), restate.WithMetadataMap(map[string]string{"k": "v"}))
```

Inspecting an error returned by a Restate operation:
```go
if te := restate.AsTerminalError(err); te != nil {
    code := te.Code()
    meta := te.Metadata()            // now a restate.StringMap, not a map[string]string
    v := meta.Get("k")               // read one value
    m := meta.ToMap()                // or get a plain map
}
```

**Note:** `ToTerminalError` does **not wrap** its argument — a `TerminalError` carries no
nested error, so `errors.Is` / `errors.As` won't reach the original through it; only the
message (`err.Error()`) is copied. There is no `NewTerminalError`.

`TerminalError.Metadata()` now returns a read-only **`restate.StringMap`** instead of
`map[string]string` — a deterministically-ordered (key-sorted) view. Use `.Get(k)`,
range `.Iter()`, or `.ToMap()` for a plain map. Set metadata with `WithMetadata(k, v)` or
`WithMetadataMap(m)` — the **same** option used for service/handler metadata.

**New:** `RetryableError` is now a public type mirroring `TerminalError` —
`ToRetryableError(err, WithErrorCode(c))` / `RetryableErrorf` / `AsRetryableError` /
`IsRetryableError`. Returning one from a handler or `Run` closure retries (like any
non-terminal error) but carries a code. Unlike `TerminalError`, it *wraps* its argument
(`errors.Is`/`As` reach through it).

`Get`, `Keys`, `Sleep`, `Wait`, and `WaitFirst` now return `restate.TerminalError`
instead of `error` (it still satisfies `error`, so most call sites are unaffected).

## Request headers

`ctx.Request().Headers` is now a read-only **`restate.StringMap`** instead of
`map[string]string` (same deterministic, key-sorted view as error metadata). Index/range
become method calls:

```go
// v0.24.0
h := ctx.Request().Headers["traceparent"]
for k, v := range ctx.Request().Headers { /* ... */ }
// 1.0
h := ctx.Request().Headers.Get("traceparent")
for k, v := range ctx.Request().Headers.Iter() { /* ... */ }
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

`Run` retry options and the invocation retry policy ([WithInvocationRetryPolicy]) now
share one vocabulary — the same builder works in both places (pass it to
`Run`/`RunAsync`/`RunVoid`, or to `WithInvocationRetryPolicy`). The `Run` names (the ones
with `Retry` in them) won, so the **invocation-policy** builders were renamed to match:

| v0.24.0 (invocation policy)         | 1.0                                  |
|-------------------------------------|--------------------------------------|
| `WithMaxAttempts(int)`              | `WithMaxRetryAttempts(uint)`         |
| `WithInitialInterval`               | `WithInitialRetryInterval`           |
| `WithMaxInterval`                   | `WithMaxRetryInterval`               |
| `WithExponentiationFactor(float64)` | `WithRetryIntervalFactor(float32)`   |

The `Run` option names are **unchanged** from v0.24.0 — and now also work inside
`WithInvocationRetryPolicy`. `WithMaxRetryDuration` stays `Run`-only;
`PauseOnMaxAttempts` / `KillOnMaxAttempts` stay invocation-policy-only.

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
