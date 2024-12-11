# protoc-gen-go-grpc

This tool generates Go language bindings of `service`s in protobuf definition
files for Restate.

An example of their use can be found in [examples/codegen](../examples/codegen)

## Usage
Via protoc:
```shell
go install github.com/restatedev/sdk-go/protoc-gen-go-restate@latest
protoc --go_out=. --go_opt=paths=source_relative \
--go-restate_out=. --go-restate_opt=paths=source_relative service.proto
```

Via [buf](https://buf.build/):
```yaml
# buf.gen.yaml
plugins:
  - remote: buf.build/protocolbuffers/go:v1.34.2
    out: .
    opt: paths=source_relative
  - local: protoc-gen-go-restate
    out: .
    opt: paths=source_relative
```

# Providing options
This protoc plugin supports the service and method extensions defined in
[proto/dev/restate/sdk/go.proto](../proto/dev/restate/sdk/go.proto).
You will need to use these extensions to define virtual objects in proto.

You can import the extensions with the statement `import "dev/restate/sdk/go.proto";`. Protoc will expect an equivalent directory
structure containing the go.proto file either locally, or under any of the
paths provided with `--proto_path`. It may be easier to use
[buf](https://buf.build/docs/bsr/module/dependency-management) to import:
```yaml
# buf.yaml
version: v2
deps:
  - buf.build/restatedev/sdk-go
```

# Upgrading from pre-v0.14
This generator used to create Restate services and methods using the Go names (eg `Greeter/SayHello`) instead of the fully qualified protobuf names (eg `helloworld.Greeter/SayHello`).
This was changed to make this package more compatible with gRPC.
To maintain the old behaviour, pass `--go-restate_opt=use_go_service_names=true` to `protoc`. With buf:
```yaml
...
  - local: protoc-gen-go-restate
    out: .
    opt:
      - paths=source_relative
      - use_go_service_names=true
```
