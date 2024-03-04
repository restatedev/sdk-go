package restate

//go:generate protoc --proto_path=proto --go_out=generated --go_opt=module=restate.dev/sdk-go/pb/service --go_opt=paths=import proto/discovery.proto proto/protocol.proto proto/dynrpc.proto
