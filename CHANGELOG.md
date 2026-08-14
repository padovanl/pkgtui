# Changelog

All notable changes to this project are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions
follow [SemVer](https://semver.org/).

## [1.3.0] - 2026-08-15

### Added

- **Disk cleanup explorer** (`K`): old kernel packages apt's own
  autoremove deliberately leaves behind, leftover config files from
  already-removed packages, and disabled snap revisions kept as a
  rollback safety net — with total reclaimable space and a direct purge.
- **"Why is this installed?"** (`W`): manual install vs. pulled in as a
  dependency, plus a navigable tree of reverse dependencies you can drill
  into.
- **apt + snap overlap view** (`O`): packages installed via both backends
  at once, and installed snaps that haven't been refreshed in 6+ months
  (using the snap's own on-disk file timestamp).
- **Unattended-upgrades dashboard** (`A`, apt): whether silent background
  upgrades are enabled, what the last automatic run touched, and when the
  next one is scheduled.
- **Install a specific version, or downgrade** (`V`, apt): pick from
  every version `apt-cache madison` knows about across configured repos.
- **Revert** (`V`, snap): restore the previous revision's binary and its
  data/config via `snap revert`.
- Redesigned the GitHub Pages landing page.

### Fixed

- The help, settings, disk cleanup and "why is this installed" screens no
  longer overflow the terminal when their content grows past one
  screenful (long reverse-dependency lists, many disk-cleanup findings,
  more keybindings) — they now scroll and keep the current selection in
  view instead of pushing their own title off-screen.

## [1.2.0] - 2026-08-14

### Fixed

- Badge text (active tab, header bar, key hints) now uses explicit
  truecolor instead of ANSI colors 0/15, fixing text that rendered gray
  instead of black/white on terminals with a customized color scheme.
- "Upgrade all" now runs `apt-get dist-upgrade` instead of plain
  `upgrade`, so it resolves dependency changes in one run instead of
  silently leaving some packages for a second pass.
- Mouse clicks below the list no longer jump the selection to an
  unrelated row.
- The "upgrade all" confirmation no longer overflows the terminal border
  on a long package list.

### Added

- `ctrl+l` forces a full screen redraw, for a stale/glitched display.

## [1.1.0] - 2026-08-14

### Added

- Real end-to-end test suite: the actual binary, a real pty, and a real
  terminal emulator (not a hand-rolled heuristic), wired into CI as a
  required check.
- A "reset keybindings to defaults" action, and an on-screen hint for
  closing the filter with `esc`.

### Fixed

- Badge text color is now picked per background instead of hardcoded
  black.

## [1.0.1] - 2026-08-14

### Added

- App icon and desktop entry, wired into both the `.deb` and `.snap`
  packages.

### Fixed

- Crash when clicking below the list, or anywhere on an empty one.
- Selected rows keep the status bullet's own color instead of blending
  into the highlight; held packages are now shown correctly in the
  Upgradable view.

### Docs

- Documented the 256-color terminal requirement.

## [1.0.0] - 2026-08-14

Initial release: browse, search, install, remove and upgrade apt and snap
packages from a single htop-style dashboard.
