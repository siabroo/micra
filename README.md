# micra

[![CI](https://github.com/siabroo/micra/actions/workflows/ci.yml/badge.svg)](https://github.com/siabroo/micra/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/siabroo/micra/core.svg)](https://pkg.go.dev/github.com/siabroo/micra/core)
[![Go Report Card](https://goreportcard.com/badge/github.com/siabroo/micra)](https://goreportcard.com/report/github.com/siabroo/micra)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A small Go library that owns service bootstrap and lifecycle: gRPC servers, Postgres pools, HTTP servers. No CLI router, no config loader, no DI. Just `App`, `Component`, `Run`, `RunOnce`.

> micra is published as a multi-module repo — install a module directly, e.g.
> `go get github.com/siabroo/micra/core@v0.1.1`.

## Modules

- `core` — `App`, `Component`, `Initializer`, `Logger`, lifecycle coordinator.
- `adapters/loggerslog` — adapts `*slog.Logger` to `core.Logger`.
- `adapters/otelinit` — installs the W3C propagator and sampler globally so a service-supplied TracerProvider can propagate `traceparent` across gRPC/HTTP boundaries.
- `adapters/otelpgx` — wraps a `*pgxpool.Pool` to prepend a Google sqlcommenter comment and emit one CLIENT span per query.
- `components/grpcserver` — gRPC server as a Component, with built-in interceptors.
- `components/grpcclient` — gRPC client connection as a Component.
- `components/pgxpool` — Postgres pool as a Component; Init does Connect+Ping.
- `components/httpserver` — `http.Server` as a Component, for `/metrics`, `/healthz`, etc.

See the package documentation on [pkg.go.dev](https://pkg.go.dev/github.com/siabroo/micra/core) for the design rationale and the full `App`/`Component` contract.
