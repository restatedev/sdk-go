package restate

//go:generate buf generate
//go:generate buf build --as-file-descriptor-set -o internal/dynrpc.binbp proto/dynrpc/dynrpc.proto
