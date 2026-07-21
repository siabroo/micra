module micra-example-redis-cache

go 1.26

require (
	github.com/redis/go-redis/v9 v9.7.3
	github.com/siabroo/micra/adapters/loggerslog v0.0.0
	github.com/siabroo/micra/components/rediscache v0.0.0
	github.com/siabroo/micra/core v0.1.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace (
	github.com/siabroo/micra/adapters/loggerslog => ../../adapters/loggerslog
	github.com/siabroo/micra/components/rediscache => ../../components/rediscache
	github.com/siabroo/micra/core => ../../core
)
