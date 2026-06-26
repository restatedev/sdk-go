# x/mocks

[testify](https://github.com/stretchr/testify) mocks for the Restate handler `Context`
and its related types, for unit-testing handlers without a running Restate runtime.

## Usage

Build a mock context, set expectations on it, and hand it to your handler with
`restate.WithMockContext`:

```go
import (
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/x/mocks"
)

func TestReserve(t *testing.T) {
	ctx := mocks.NewMockContext(t)
	ctx.EXPECT().GetAndReturn("status", "available")
	ctx.EXPECT().Set("status", "reserved")

	ok, err := (&ticketService{}).Reserve(restate.WithMockContext(ctx), restate.Void{})
	// ...
}
```

### Controlling randomness

`restate.Rand` / `restate.UUID` / `restate.RandSource` return concrete, deterministically
seeded values, so they are not mocked as interfaces. Instead:

- `ctx.EXPECT().WithRandSeed(seed)` — makes `Rand`/`UUID`/`RandSource` produce deterministic
  output derived from `seed`, mirroring how the runtime seeds randomness from the invocation id.
- `ctx.EXPECT().RandUUID().Return(id)` — force a specific UUID.

## Regenerating the mocks

From the **repo root**:

```shell
.tools/gen-mocks.sh
```

Do **not** run `mockery` directly and commit its raw output — it won't compile. `mockery`
(config in `x/mocks/.mockery.yaml`) emits a full `type MockX struct { mock.Mock }` plus a
constructor for every interface, but this package needs two hand-maintained deviations that
live in `helpers.go`:

- The future mocks — `MockResponseFuture`, `MockRunAsyncFuture`, `MockAttachFuture`,
  `MockAwakeableFuture`, `MockDurablePromise`, `MockAfterFuture` — must **embed
  `restatecontext.Future`** to satisfy its unexported `handle()` method (a mock in this
  package cannot implement an unexported method from another package).
- `MockContext` / `MockClient` use richer hand-written constructors (they register `t`).

`gen-mocks.sh` runs `mockery` and then strips the colliding generated struct definitions and
constructors so the `helpers.go` versions win, and finally re-builds the module to confirm it
compiles. See the script's header comment for details.
