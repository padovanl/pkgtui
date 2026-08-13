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
