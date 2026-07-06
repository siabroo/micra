# Releasing micra

micra is a multi-module repo; each module is tagged independently as
`<module-path>/vX.Y.Z` (e.g. `core/v0.1.0`, `components/pgxpool/v0.1.0`).

Because dependents require `core`, release in dependency order:

1. Tag and push `core` first: `core/vX.Y.Z`.
2. In each core-dependent module (`adapters/loggerslog`,
   `components/grpcclient`, `components/grpcserver`, `components/pgxpool`),
   ensure `go.mod` requires `github.com/siabroo/micra/core@vX.Y.Z` with **no**
   `replace` directive, then `go mod tidy`. Commit.
3. Tag and push every remaining module at that commit.
4. Independent modules (`adapters/otelinit`, `adapters/otelpgx`,
   `components/httpserver`) carry no `core` dependency and can be tagged in
   step 3 directly.
5. Create a GitHub Release per tag.

`just release VERSION=v0.1.0` automates the tag-and-push dance (it does NOT
edit go.mod files — do that as a reviewed commit first).
