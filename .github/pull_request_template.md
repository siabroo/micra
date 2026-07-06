## Why
<!-- The reason for this change: the problem, motivation, or goal. -->

## What
<!-- What was actually done — the concrete changes, module by module. -->

## How tested
- [ ] `just test` (unit)
- [ ] `just test-integration` (if touching pgxpool/otelpgx)
- [ ] `just lint`
- [ ] `go test -race ./...` in touched modules (concurrency/lifecycle/servers)

## Documentation currency
- [ ] godoc updated on any added/changed exported symbols
- [ ] README / CONTRIBUTING / RELEASING updated where relevant
- [ ] inline comments near changed code are still accurate (no stale/lying comments)

## Notes for reviewers
