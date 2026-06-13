# AGENTS.md — operating guide for AI coding agents

Applies to Claude Code, Codex, and any other agent working in this repository.

## Memory protocol (do this every session)

1. **On start:** read [`MEMORY.md`](MEMORY.md) before reading or editing code. It
   is the architecture map, the list of invariants you must not break, and the
   dev/build workflow.
2. **On change:** whenever you alter architecture, a package boundary, a public
   contract, an invariant, or the build/dev workflow, **update `MEMORY.md` in
   the same commit**. Treat a stale `MEMORY.md` as a bug. Keep it a concise map,
   not a changelog.
3. **When unsure** whether something is load-bearing, check `MEMORY.md`'s
   "Invariants & design tenets" section first.

## How to work here

- Language/build: Go 1.26. Run `make verify` (vet + golangci-lint v2.12.2 +
  tests) before declaring done. **Tests must pass under `go test -race ./...`** —
  the install path is concurrent.
- Match the surrounding style: small packages, function-typed DI seams (not
  interfaces), errors wrapped with `%w`, table-driven tests with `t.TempDir`,
  injected `*http.Client` for network code.
- Security boundary: family names must go through `internal/fontname.Validate`;
  never reintroduce a second copy of that guard.
- Commits: Conventional Commits with a `Co-Authored-By` trailer; branch off
  `main`; only push/PR when asked.

## Skills

Reusable, vendored skills live under `.agents/skills/` (the `golang-*` set plus
testing/security/review). The end-to-end review methodology used to harden this
codebase is captured in
[`.agents/skills/principal-review-and-refactor`](.agents/skills/principal-review-and-refactor/SKILL.md):
fan out concern-scoped reviewers, synthesize one prioritized plan, then
implement it as small, individually-green commits.

## Quick reference

| Task | Command |
|------|---------|
| Build | `go build ./...` |
| Test | `go test ./...` |
| Test (race) | `go test -race ./...` |
| Vet | `go vet ./...` |
| Lint | `golangci-lint run` |
| Everything | `make verify` |
| Run | `go run ./cmd/nerdfont-install --help` |
