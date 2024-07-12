module github.com/restatedev/sdk-go

go 1.22.0

require (
	github.com/google/uuid v1.6.0
	github.com/posener/h2conn v0.0.0-20231204025407-3997deeca0f0
	github.com/stretchr/testify v1.9.0
	github.com/vmihailenco/msgpack/v5 v5.4.1
	golang.org/x/net v0.21.0
	google.golang.org/protobuf v1.32.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/restatedev/sdk-go/generated/dev/restate => ./generated/dev/restate
