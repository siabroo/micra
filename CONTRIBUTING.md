# Contributing to micra

## Development process

All changes land through a branch and a Pull Request into `main` (never direct
commits), with a **Why / What** description and adversarial + documentation-currency
review before merge. The full process — followed by both humans and AI agents —
is in [`AGENTS.md`](AGENTS.md).

## Local development

micra is a multi-module Go repo. A root `go.work` lists every module, so
`go test ./...` inside any module resolves cross-module deps locally without
`replace` directives.

```sh
just test              # hermetic unit tests across all modules
just test-integration  # Docker-backed tests (otelpgx, pgxpool) — needs Docker
just lint              # golangci-lint across all modules
just tidy              # go mod tidy across all modules
```

## Adding a new Component

1. Create `components/<name>/go.mod` declaring `github.com/siabroo/micra/components/<name>`.
2. Add `./components/<name>` to the root `go.work`.
3. Implement the `core.Component` interface. Implement `core.Initializer` if the component has a synchronous setup phase (Connect, Ping, Listen-only).
4. Tests sit next to the source. Integration tests for components touching external services (Postgres, MinIO, S3) use `testcontainers-go`.

## Stability

- v0.x: breaking changes allowed between minor versions, called out in commit messages.
- v1.0: API frozen.
