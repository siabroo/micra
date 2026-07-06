# Working on micra (process for AI agents)

This file is the authoritative development process for micra. AI agents working
in this repo MUST follow it. Humans: see also `CONTRIBUTING.md`.

micra is a multi-module Go library (`core`, `adapters/*`, `components/*`).
`main` is always releasable and green.

## Golden rules

1. **Never commit directly to `main`.** Every change lands through a branch and
   a Pull Request. (Branch protection will enforce this once the repo is public;
   until then it is a hard convention — follow it anyway.)
2. **`main` must stay green.** A PR merges only when CI passes and review is done.
3. **One logical change per PR.** Keep PRs small and focused.

## Branch & PR flow

1. Branch off `main`: `feat/…`, `fix/…`, `chore/…`, or `docs/…`.
2. Implement the change (see "How to write code").
3. Open a PR into `main` with a description that has **Why** and **What**
   sections (see "PR description").
4. Run the review (see "How to review"). Address findings.
5. The maintainer gives final approval and squash-merges; delete the branch.

## How to write code

- **TDD.** Write a failing test first, then the minimal code to pass it.
- **Race-sensitive code** (lifecycle, concurrency, servers) is tested with
  `go test -race`.
- **Follow existing patterns**: functional `With…` options, the `Init`/`Start`/
  `Stop` component lifecycle, and the interceptor style already in the tree.
- **Secure, sane defaults.** A library shipping an insecure default is a bug.
- Before finishing: `just test` (hermetic), `just lint` (0 issues), and
  `go test -race ./...` in any module you touched must all pass.

## Versioning & releases

- Decide the version bump **from the nature of the changes** (patch/minor/major),
  per `RELEASING.md` → "Choosing the version bump". Importance decides *whether
  to release now*, not the number.
- Tag only the modules whose code changed; release in dependency order.

## Design documents vs. plans

- **Keep design docs.** Specifications / design rationale live in
  `docs/superpowers/specs/` and ARE committed — they record *why* and *what*.
  Write one only for substantial or architectural work; for small changes the
  PR's Why/What is enough.
- **Do not commit detailed step-by-step implementation plans.** They are
  ephemeral scaffolding, go stale immediately, and add noise. `docs/superpowers/
  plans/` is git-ignored; keep plans there or in scratch, never in a PR.

## PR description (required format)

Every PR body must contain:

```
## Why
<the reason for the change — the problem, motivation, or goal>

## What
<what was actually done — the concrete changes, module by module>

## How tested
<commands run and results: just test, just lint, go test -race for touched modules>
```

The Why/What is the durable record of intent — it outlives any plan.

## How to review (before merge)

Reviews are adversarial and read the **actual code**, not just the diff.

1. **Correctness & quality review.** A reviewer (fresh agent) checks the change
   against its stated intent: correctness, edge cases, tests that actually
   assert, no scope creep. Default to skeptical; verify claims from the code.
2. **Security-sensitive changes** (auth, transport, input handling, concurrency)
   get an extra skeptical pass that tries to *refute* the change's safety.
3. **Documentation-currency review (required).** A reviewer verifies the change
   is reflected in:
   - godoc on any added/changed **exported** symbols,
   - `README.md`, `CONTRIBUTING.md`, `RELEASING.md` where relevant,
   - inline comments near the changed code — no stale or now-incorrect comments.
   A comment that lies about the code is a defect and blocks merge.
4. Findings are fixed and re-reviewed until clean. Then the maintainer approves.
