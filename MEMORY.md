# Project Memory — nerd-font-installer

> **Protocol for all agents (Claude, Codex, others):** Read this file at the
> start of every session *before* touching code. After any change that alters
> architecture, a package boundary, a public contract, an invariant, or the
> dev/build workflow, **update this file in the same change**. Keep it short and
> current — it is a map, not a changelog. See [AGENTS.md](AGENTS.md).

## What this is

`nerdfont-install` is a small, scriptable Go CLI that installs
[Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) from a YAML config or an
interactive TUI: it resolves a release, downloads per-family zip archives from
GitHub, extracts the font files into the user's font directory, and refreshes
the font cache. Audience: dotfiles / fresh-machine / dev-container setup.

## Architecture (package map)

- `cmd/nerdfont-install` — entrypoint & orchestration.
  - `run(ctx, args, stdin, stdout, stderr, deps) int` is the real, testable main;
    `main()` only wires os values into it.
  - Dependency injection via the `dependencies` struct of **function-typed
    fields** (`loadConfig`, `discoverConfig`, `listReleases`, `runTUI`,
    `installFonts`, `isTerminal`) + `withDefaults()`. This is deliberately NOT
    an interface seam — keep it that way for a CLI this size.
  - Exit-code contract (`exitCodeFor`): `0` success/cancelled, `2`
    user-correctable input (missing config, unknown/absent release), `1`
    runtime failure (network/fs/install).
  - `version/commit/date` are injected via `-ldflags -X main.*`.
- `internal/config` — YAML loading. Strict decode (`KnownFields(true)`),
  `ApplyDefaults` → `Normalize` → `Validate`. `Discover`/`DefaultPaths` search
  user + XDG (`os.UserConfigDir`) + binary-dir locations.
- `internal/nerdfonts` — GitHub releases API client. Zero-value-with-defaults
  `Client` (configurable `HTTPClient`/`BaseURL`/`MaxPages`). Owns `Latest`
  const and typed errors `ErrNoReleases`, `ReleaseNotFoundError`. Pagination
  stops on an empty *raw* API page, not a filtered-empty one.
- `internal/fonts` — the install engine. Per family: download → extract → atomic
  replace → optional `fc-cache`. Families install **concurrently** (errgroup,
  limit `maxConcurrentInstalls`); a `syncWriter` keeps progress lines from
  interleaving. Size caps (`maxDownloadBytes`/`maxFontFileBytes`/
  `maxArchiveBytes`) bound zip bombs.
- `internal/fontname` — the single shared path-traversal validator
  (`Validate`), used at both trust boundaries (`config` and `fonts`). Security
  boundary — do not duplicate it.
- `internal/tui` — Bubble Tea picker: release step → families step, with
  `IconMode` (auto/nerd/unicode/ascii) icon sets.
- `snap/snapcraft.yaml` — Snapcraft packaging for the CLI. It builds the Go
  command with the same ldflags contract and uses classic confinement so the
  tool can write real user font directories and refresh fontconfig.

## Invariants & design tenets (do not break)

1. **Family-name trust boundary:** every family name passes
   `fontname.Validate` before it is joined onto a path or URL. One
   implementation, used everywhere.
2. **Atomic per-family install:** stage into a unique temp dir, `rename` into
   place, keep a `.old` backup. Disjoint per-family paths are what make
   concurrency safe — preserve that if you touch the install loop.
3. **Best-effort cleanup must never fail a committed operation** (e.g. removing
   the `.old` backup after a successful swap).
4. **Surface write-close/flush errors** on written font files (`Sync` + explicit
   `Close`) — a swallowed close can promote a truncated font.
5. **Testability:** `run` takes explicit I/O; HTTP goes through an injectable
   `*http.Client` (tests use `roundTripFunc` + in-memory zips, or `httptest`).
6. **Security defaults:** strict YAML, `url.PathEscape` on URL segments,
   `exec.CommandContext` (no shell) for `fc-cache`, size caps on all copies.
7. **Download integrity:** each zip is verified against the release's
   `SHA-256.txt` manifest (`fetchChecksums` → per-family digest). Verification
   is *best-effort*: a missing manifest warns and proceeds; a digest **mismatch
   aborts** the install. Do not weaken the mismatch-is-fatal rule.

## Dev / build workflow

- Go **1.26**. Module: `github.com/w0rxbend/nerd-font-installer`.
- `make verify` ≈ `go vet ./...` + `golangci-lint run` (v2.12.2) + `go test ./...`.
  `golangci-lint` is a required check for every code change; `make lint` is the
  focused lint target. **Tests must pass under `-race`** (the install path is
  concurrent).
- CI runs vet, lint, test matrix (ubuntu+macOS), race, coverage, govulncheck,
  actionlint. Mirror that locally before pushing.
- Release CI publishes versioned GitHub releases for `v*` tags and refreshes a
  moving `latest` release with stable asset names for fixed download URLs.
- Snap CI builds snaps on PRs/main/tags/manual runs. Non-PR runs publish to the
  Snap Store (`main` → `edge`, `v*` tags → `stable`, manual → chosen channel)
  using the `SNAPCRAFT_STORE_CREDENTIALS` repository secret. The snap name must
  be registered and classic confinement approved before public stable release.
- Commits: Conventional Commits + `Co-Authored-By` trailer. Branch off `main`.
- Reusable Go skills live under `.agents/skills/` (golang-*, testing, security,
  review). The review→refactor methodology is the
  `principal-review-and-refactor` skill.

## Open / deferred decisions (need a human call)

- **Release tooling duplication:** `.goreleaser.yaml` and the hand-rolled bash
  in `.github/workflows/release.yml` are two sources of truth — pick one.
- **Config filename stems** are inconsistent (`.nerd-config.yaml`,
  `nerd-config-installer/`, `config.yaml`, binary name `nerdfont-install`).
  Standardize additively (keep old paths as fallbacks) to avoid relocating
  existing users' files.
