# Contributing to pkgtui

Thanks for considering a contribution! A few notes to make it smooth.

## Branching

- `develop` is where active work happens — open pull requests against it,
  not `main`.
- `main` only receives merges from `develop` when a release is ready;
  tagging `main` with `vX.Y.Z` triggers the release pipeline (see
  `.github/workflows/release.yml`). CI on `main` won't accept a tag unless
  it's reachable from `main`, so there's no way to accidentally ship
  something that skipped `develop`.

## Getting set up

```bash
git clone https://github.com/padovanl/pkgtui.git
cd pkgtui
git checkout develop
go build -o pkgtui .
./pkgtui
```

Go 1.24+ is required (see `go.mod`).

## Before opening a PR

```bash
gofmt -l .      # should print nothing
go vet ./...
go test ./...
go build ./...
```

All four run in CI on every push and PR; a red CI check is the fastest way
to get a review request bounced back to you, so it's worth running them
locally first.

### End-to-end tests

`go test ./...` only covers unit tests and tests that call UI methods
directly (`internal/ui/panel_test.go` and friends) — none of those touch
bubbletea's actual render pipeline, so a bug that only exists in what's
really drawn to the terminal can slip past them. `e2e/` closes that gap:
it builds the real binary and drives it through a real pty, feeding the
output into an actual terminal emulator ([hinshun/vt10x](https://github.com/hinshun/vt10x))
and asserting on the resulting screen content, not on internal state.

```bash
go test -tags e2e ./e2e/...
```

Excluded from the plain `go test ./...` above via its build tag (it
compiles a fresh binary and needs a real pty, so it's slower), but runs
as its own required CI step — same as the rest, a red check there blocks
merging.

When adding a UI regression test, prefer `internal/ui/panel_test.go`'s
style (call `Panel.handleKey`/`handleMouse` directly) when the bug is in
application *logic*; reach for `e2e/` when it's specifically about what
ends up on screen — as `e2e/settings_test.go`'s own history shows, a
hand-rolled "reconstruct the current frame from raw bytes" approach is
an easy way to fool yourself into a false failure (or a false pass):
bubbletea redraws incrementally, and only a real emulator reliably knows
what's actually on screen at any given moment.

## Code style

- Comments explain *why*, not *what* — skip a comment if the code already
  says it.
- No new abstractions/config knobs for a single call site; three similar
  lines beat a premature helper.
- `internal/apt` and `internal/snap` keep command-execution (`exec.Command`)
  separate from output parsing (plain functions taking a string, returning
  parsed data) — see `parseUpgradableOutput` for the pattern. This is what
  makes the parsers unit-testable without a real apt/snap on the test
  machine; new parsing logic should follow the same split, with tests using
  captured real command output as fixtures.

## Adding a feature that only makes sense for one backend

apt and snap don't map onto each other 1:1 (autoremove, dependency trees and
size accounting are apt-only; channels are snap-only). Rather than adding
no-op methods to the other backend, we use small optional interfaces in
`internal/pkg/pkg.go` (`OrphanLister`, `ChannelInstaller`, `BatchManager`)
that a backend implements only if it applies, and the UI type-asserts for
in `internal/ui/panel.go`. Follow that pattern for backend-specific
features rather than growing the core `Manager` interface.

## Reporting bugs / requesting features

Use the issue templates — they ask for the details that usually turn into
follow-up questions anyway (pkgtui version, distro, exact command run).

## Releasing (maintainers)

This section only applies if you have push access to `main` — regular
contributors don't need any of this.

The project uses [goreleaser](https://goreleaser.com) to build multi-arch
binaries and generate the `.deb` (via its built-in nfpm integration), and
[snapcraft](https://snapcraft.io/docs/snapcraft-overview) for the `.snap`
(see `snap/snapcraft.yaml`).

```bash
# Local build + .deb, without publishing anything
goreleaser release --snapshot --clean --skip=publish

# Local snap package
snapcraft pack
```

Development happens on `develop`; releases are cut from `main`. Every push
to either branch runs the [CI workflow](.github/workflows/ci.yml)
(`gofmt`, `go vet`, `go build`, `go test`). To ship a release: merge
`develop` into `main`, then run:

```bash
git checkout main && git merge develop && git push origin main
scripts/release.sh vX.Y.Z
```

`scripts/release.sh` refuses to run unless you're on `main`, the working
tree is clean, local `main` matches `origin/main`, and the tag doesn't
already exist locally or on the remote — then it tags and pushes. That
push triggers the [release workflow](.github/workflows/release.yml), which
independently re-checks the tag is on `main`, then builds and attaches the
`.deb`, `.snap` and `.tar.gz` to a new GitHub Release.
