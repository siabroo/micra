module micra-example-grpc-server

go 1.26

require (
	github.com/siabroo/micra/adapters/loggerslog v0.0.0
	github.com/siabroo/micra/components/grpcserver v0.0.0
	github.com/siabroo/micra/core v0.1.0
	google.golang.org/grpc v1.74.2
)

require (
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

// Local development: resolve micra modules to in-tree paths.
replace (
	github.com/siabroo/micra/adapters/loggerslog => ../../adapters/loggerslog
	github.com/siabroo/micra/components/grpcserver => ../../components/grpcserver
	github.com/siabroo/micra/core => ../../core
)
