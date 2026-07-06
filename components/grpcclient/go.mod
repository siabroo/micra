module github.com/siabroo/micra/components/grpcclient

go 1.26

require (
	github.com/siabroo/micra/core v0.0.0
	google.golang.org/grpc v1.74.2
)

require (
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

// In-tree replace for the unreleased core module. Remove this line
// when extracting micra to its own repo and using a tagged version.
replace github.com/siabroo/micra/core => ../../core
