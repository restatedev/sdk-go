package restate

//disabled go:generate protoc  --proto_path=proto --go_out=generated --go_opt=module=restate.dev/sdk-go/pb/service --go_opt=paths=import proto/discovery.proto proto/protocol.proto proto/dynrpc.proto

//go:generate buf generate
//go:generate buf build --as-file-descriptor-set -o generated/dynrpc.binbp proto/dynrpc/dynrpc.proto
