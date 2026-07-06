# Contributing to micra

## Local development

micra v0.1 develops in-tree inside the nestjs-one monorepo. All micra modules are listed in the root `go.work`, so `go test ./...` inside any module resolves cross-module deps locally without `replace` directives.

```sh
just test           # run all module tests
just lint           # run go vet + staticcheck across modules
```

## Adding a new Component

1. Create `libs/micra/components/<name>/go.mod` declaring `github.com/siabroo/micra/components/<name>`.
2. Add `./libs/micra/components/<name>` to the root `go.work`.
3. Implement the `core.Component` interface. Implement `core.Initializer` if the component has a synchronous setup phase (Connect, Ping, Listen-only).
4. Tests sit next to the source. Integration tests for components touching external services (Postgres, MinIO, S3) use `testcontainers-go`.

## Stability

- v0.x: breaking changes allowed between minor versions, called out in commit messages.
- v1.0: API frozen.
