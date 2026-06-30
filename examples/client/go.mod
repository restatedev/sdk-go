module github.com/restatedev/sdk-go/examples/client

go 1.25.0

require (
	github.com/restatedev/sdk-go v0.9.1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.64.0
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	github.com/restatedev/sdk-go => ../../
	github.com/restatedev/sdk-go/ingress => ../../ingress
	github.com/restatedev/sdk-go/server => ../../server
)
