module micra-example-multi-mode

go 1.26

require (
	github.com/siabroo/micra/adapters/loggerslog v0.0.0
	github.com/siabroo/micra/components/grpcserver v0.0.0
	github.com/siabroo/micra/components/pgxpool v0.0.0
	github.com/siabroo/micra/core v0.1.0
	google.golang.org/grpc v1.74.2
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace (
	github.com/siabroo/micra/adapters/loggerslog => ../../adapters/loggerslog
	github.com/siabroo/micra/components/grpcserver => ../../components/grpcserver
	github.com/siabroo/micra/components/pgxpool => ../../components/pgxpool
	github.com/siabroo/micra/core => ../../core
)
