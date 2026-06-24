# Releasing

Multi-module repo. Each module is published with a Git tag prefixed by its subdirectory;
`go get …@<version>` resolves it. `.tools/release.sh` does the tagging.

| Module | Tag | Line |
|---|---|---|
| `github.com/restatedev/sdk-go` (main) | `vX.Y.Z` | 1.x |
| `…/testing` | `testing/vX.Y.Z` | 1.x |
| `…/x/mocks` | `x/mocks/vX.Y.Z` | 0.x |
| `…/x/protoc-gen-go-restate` | `x/protoc-gen-go-restate/vX.Y.Z` | 0.x |

Not published: `examples/*`, `test-services`.

## Model

- The main module is always tagged; **submodules are released only when you pass
  `<submodule>=<version>`** (no lockstep).
- `testing` and `x/mocks` import the SDK, so their `go.mod` require is pinned to the
  released SDK version and committed before tagging. `x/protoc-gen-go-restate` has no SDK
  dep — tag only.
- `x/mocks` reaches into `internal/*`: when a release changes those, re-cut `x/mocks`
  re-pinned to the new SDK.

## Usage

```sh
.tools/release.sh <sdk-version> [<submodule>=<version> ...] [--push]

.tools/release.sh v1.0.0 testing=v1.0.0 x/mocks=v0.1.0 x/protoc-gen-go-restate=v0.1.0  # full first release
.tools/release.sh v1.0.1                                                               # SDK-only patch
.tools/release.sh v1.1.0 x/mocks=v0.1.1                                                # re-cut mocks for new internals
```

Nothing is pushed without `--push`. When you tag `x/protoc-gen-go-restate`, also publish
its BSR contract: `( cd x/protoc-gen-go-restate && buf push )` (the script reminds you).