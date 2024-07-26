# protoc-gen-go-grpc

This tool generates Go language bindings of `service`s in protobuf definition
files for Restate.

## Usage
Via protoc:
```shell
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
  - local:
      - docker
      - run
      - --pull=always
      - -i
      - ghcr.io/restatedev/protoc-gen-go-restate:latest
    out: .
    opt: paths=source_relative
```

# Building a docker image
```
KO_DOCKER_REPO=ghcr.io/restatedev ko build --platform=all -B
```
