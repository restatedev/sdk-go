#!/usr/bin/env bash
#
# Rebuilds the generated "core" artifacts of the SDK:
#
#   1. the shared-core Rust crate -> WebAssembly, installed into
#      internal/statemachine/ (where it is //go:embed-ed by the SDK).
#   2. the protobuf Go code, via buf:
#        - the internal wire protocol  -> internal/generated
#        - the codegen contract        -> x/protoc-gen-go-restate/generated
#
# Requirements:
#   - Rust toolchain with the wasm32-unknown-unknown target
#       rustup target add wasm32-unknown-unknown
#   - buf (https://buf.build/docs/installation) — may need network for remote plugins.
set -euo pipefail
cd "$(dirname "$0")/.."

WASM=shared_core_golang_wasm_binding.wasm

command -v cargo >/dev/null 2>&1 || { echo "error: cargo not found on PATH (install the Rust toolchain)" >&2; exit 1; }
command -v buf   >/dev/null 2>&1 || { echo "error: buf not found on PATH (https://buf.build/docs/installation)" >&2; exit 1; }

echo "==> building shared-core -> wasm"
( cd internal/shared-core && cargo build --release )
cp "internal/shared-core/target/wasm32-unknown-unknown/release/$WASM" "internal/statemachine/$WASM"
echo "    installed internal/statemachine/$WASM"

echo "==> generating internal protobuf (internal/generated)"
buf generate --template internal.buf.gen.yaml

echo "==> generating codegen contract (x/protoc-gen-go-restate/generated)"
( cd x/protoc-gen-go-restate && buf generate )

echo "done"
