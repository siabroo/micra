# micra

A small Go library that owns service bootstrap and lifecycle: gRPC servers, Postgres pools, HTTP servers. No CLI router, no config loader, no DI. Just `App`, `Component`, `Run`, `RunOnce`.

> v0.1 lives in-tree at `libs/micra/` inside the [nestjs-one monorepo](https://github.com/siabroo/nestjs-one). It will be extracted to `https://github.com/siabroo/micra` after 1–2 iterations of in-tree use. Import paths already use `github.com/siabroo/micra/...` so the extraction is a directory move.

## Modules

- `core` — `App`, `Component`, `Initializer`, `Logger`, lifecycle coordinator.
- `adapters/loggerslog` — adapts `*slog.Logger` to `core.Logger`.
- `adapters/otelinit` — installs the W3C propagator and sampler globally so a service-supplied TracerProvider can propagate `traceparent` across gRPC/HTTP boundaries.
- `adapters/otelpgx` — wraps a `*pgxpool.Pool` to prepend a Google sqlcommenter comment and emit one CLIENT span per query.
- `components/grpcserver` — gRPC server as a Component, with built-in interceptors.
- `components/grpcclient` — gRPC client connection as a Component.
- `components/pgxpool` — Postgres pool as a Component; Init does Connect+Ping.
- `components/httpserver` — `http.Server` as a Component, for `/metrics`, `/healthz`, etc.

See `docs/superpowers/specs/2026-06-04-go-service-runtime-design.md` and `docs/superpowers/specs/2026-06-05-micra-otel-sqlcommenter-design.md` (in the monorepo) for the design rationale.
