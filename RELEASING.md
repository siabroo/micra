# Releasing micra

micra is a multi-module repo; each module is tagged independently as
`<module-path>/vX.Y.Z` (e.g. `core/v0.1.0`, `components/pgxpool/v0.1.0`).

## Choosing the version bump (do this first)

**Before every release, decide the new version by assessing the changes that
are actually going in — never bump by habit or by how "important" a change
feels.** The bump is driven by the *nature* of the change, per SemVer:

- **patch** (`vX.Y.Z+1`) — bug fixes and internal changes only. No new or
  changed public API, no behavior change consumers can observe.
- **minor** (`vX.Y+1.0`) — new backward-compatible public API or added
  functionality. Existing code keeps compiling and behaving.
- **major** (`vX+1.0.0`) — a breaking change: removed/renamed exported
  symbols, changed signatures, or behavior that can break existing callers.
  Note: a Go **v2+** major also changes the module path (`/v2` suffix) — avoid
  until genuinely needed.

Rules of thumb specific to micra:

- **Importance ≠ version size.** A critical bug fix is still a *patch*; a small
  new option is still a *minor*. Urgency decides *whether to release now*, not
  the number.
- **Pre-1.0 (`v0.x`).** We are on `0.x`, where the API is still unstable. Added
  functionality is a minor (`0.1 → 0.2`); breaking changes are *allowed* within
  `0.x` but should still bump the minor and be called out in the release notes.
  `v1.0.0` is a deliberate stability commitment — cut it on purpose, not by
  accumulation.
- **Bump only the modules that changed.** In this multi-module repo, tag just
  the modules whose code changed; untouched modules keep their current tag.
  When a bumped module is depended on by others (e.g. `core`), release it first
  and let dependents pick it up (see the dependency-ordered steps below).
- **When unsure, diff the public API** (`go doc ./...` before vs. after, or read
  the exported surface): any addition → minor; any removal/change → major;
  neither → patch.

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
