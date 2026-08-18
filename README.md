# pkgtui 📦

[![CI](https://github.com/padovanl/pkgtui/actions/workflows/ci.yml/badge.svg)](https://github.com/padovanl/pkgtui/actions/workflows/ci.yml)
[![Release](https://github.com/padovanl/pkgtui/actions/workflows/release.yml/badge.svg)](https://github.com/padovanl/pkgtui/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/padovanl/pkgtui?sort=semver)](https://github.com/padovanl/pkgtui/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/padovanl/pkgtui/total)](https://github.com/padovanl/pkgtui/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/padovanl/pkgtui)](go.mod)
[![License: MIT](https://img.shields.io/github/license/padovanl/pkgtui)](LICENSE)

An **htop-style** terminal UI (TUI) to search, install, remove and upgrade
**apt** and **snap** packages, from a single dashboard, without having to
remember the syntax of either package manager. See [CHANGELOG.md](CHANGELOG.md)
for what's new.

![pkgtui apt demo](assets/demo.gif)
*Searching, inspecting and installing an **apt** package, live.*

![pkgtui snap demo](assets/demo-snap.gif)
*Same flow on **snap**, including the channel picker (`c` to cycle stable/candidate/beta/edge).*

## ✨ Features

### 🔍 Browse & search

- **Two separate backends**: dedicated tabs for `apt` and `snap`, each with
  its own state (the two worlds are never mixed together).
- **Live search**: search packages by name/description (`apt-cache search`
  / `snap find`).
- **Local filter**: narrow whatever list is currently on screen as you type
  — instant, no external query.
- **Installed / Upgradable / Orphaned**: dedicated views for what's on your
  system, what has an update, and (apt) what `apt-get autoremove` would
  clean up.
- **Package details**: description, version, dependencies — including
  *reverse* dependencies, so you know what else relies on a package before
  you remove it — or publisher/channels for snap.
- **Sort by installed size**: find what's actually eating your disk.
- **Mouse support**: click a tab to switch backend, click a row to select
  it, scroll to navigate.

### ⚡ Act on packages

- **Install/remove/upgrade with confirmation**: every privileged action
  asks for confirmation (`y`/`n`) before running.
- **Live output, without losing the app**: `apt-get`/`snap` commands run
  in a real pseudo-terminal shown in a bordered box inside pkgtui — you
  still get the genuine `sudo` password prompt and any interactive
  dpkg/debconf dialog, but the screen doesn't get handed over. The view
  stays up after the command finishes (green border on success, red on
  failure) until you press a key, so you can actually read the result.
- **Multi-select**: tag several packages and install/remove them in one
  batch action.
- **Upgrade all, with a preview**: `U` fetches and shows exactly which
  packages will change before you confirm, instead of a blind "upgrade
  everything?".
- **Security updates called out**: upgradable packages coming from a
  `-security` repo get a distinct red marker, and the upgrade-all
  confirmation counts them separately.
- **Channel picker**: install a snap from `stable`, `candidate`, `beta` or
  `edge` instead of always defaulting to stable.
- **Hold/pin (apt)**: block a package from being touched by upgrades
  (`apt-mark hold`), with held packages marked `[held]` in the list.
- **Changelog viewer (apt)**: see what actually changed in a package
  before upgrading it (`apt-get changelog`).
- **PPA management (apt)**: list, add and remove third-party repositories
  from inside the TUI, gated behind an explicit warning since a bad PPA
  can break `apt update` for the whole system.
- **Install a specific version, or downgrade (`V`, apt)**: pick from every
  version `apt-cache madison` knows about across your configured repos,
  not just the one candidate apt would offer on its own — useful right
  after an upgrade turns out to be the one you wanted to avoid.
- **Revert (`V`, snap)**: snap's own idiomatic undo — restores the
  previous revision's binary *and* its data/config, not just an older
  version number.

### 🧹 Cleanup & insight

These go beyond wrapping `apt`/`snap` commands — they answer questions
neither tool (nor any TUI wrapping them) normally surfaces on its own:

- **Disk cleanup explorer (`K`)**: old kernel packages apt's own
  `autoremove` deliberately leaves behind, leftover config files from
  already-removed packages (`dpkg`'s "rc" state — never cleaned up
  automatically), and disabled old snap revisions kept as a rollback
  safety net nobody ever revisits. Shows total reclaimable space, purge
  one finding at a time with the same confirm-then-run flow as every
  other privileged action.
- **Dependency tree (`W`)**: whether the selected package was explicitly
  asked for or only pulled in as a dependency, plus a navigable
  `├──`/`└──` tree of what currently depends on it, with a breadcrumb
  trail — drill into any reverse dependency to ask the same question
  about *it*, instead of piping `apt-cache rdepends` through your own
  head.
- **apt+snap overlap view (`O`)**: packages installed via *both* backends
  at once (Canonical has, at times, silently substituted apt packages
  like Firefox with a snap "transitional" package — this is how you'd
  actually notice), and installed snaps that haven't been refreshed in
  6+ months, using the snap's own on-disk file timestamp rather than any
  locale-dependent text. apt already flags upgradable packages; snap has
  no equivalent signal for "this has sat untouched for years," which
  real-world surveys of installed snaps have found is common.
- **Unattended-upgrades dashboard (`A`, apt)**: whether silent background
  upgrades are even enabled, what the last automatic run actually
  touched, and when the next one is scheduled — normally visible only by
  digging through `/var/log/unattended-upgrades/` by hand.
- **Metrics dashboard (`M`)**: installed packages ranked by disk usage as
  a bar chart, for both backends (snap has no size column of its own in
  `snap list`, so pkgtui reads it straight off the installed revision's
  file on disk instead).
- **Upgrade conflicts (`X`, apt)**: packages a conservative `apt-get
  upgrade` would leave behind because it needs to install or remove
  something else first — distinct from an explicit hold, which already
  has its own `◆` marker. "Upgrade all" (`U`) already resolves most of
  these on its own (it uses `dist-upgrade`); this is what tells you
  *which* ones and *why*.
- **Action log (`L`)**: every privileged action run this session, both
  backends together, with a timestamp and success/failure — a running
  record of what you actually did, without digging through shell
  history.

### 🎨 Make it yours

- **11 built-in color themes**: from classic terminal looks (Dracula,
  Nord, Solarized, Gruvbox) to Catppuccin, Tokyo Night, Monokai, Darcula,
  VS Code Dark+ and Ubuntu's own palette. Browse them with `←`/`→` in the
  settings screen — the whole UI re-skins live as you move, no need to
  commit to one just to see it.
- **Rebind any action key**, live, from inside the app — no config file
  editing required (though it's saved to one), with a one-key reset back
  to the defaults if a round of rebinding goes sideways.
- **Remembers where you left off**: backend and view are restored on the
  next launch.

## 📥 Installation

No verification/store required: grab the asset from the [releases
page](https://github.com/padovanl/pkgtui/releases) and install it locally.
The `.deb` and `.snap` both register an application entry and icon, so
pkgtui shows up in your desktop's app launcher (opening in a terminal,
since it's a TUI) if you have one — a headless install just ignores it.

Below, `<version>` means the release number **without** a leading `v` (a
`v0.1.1` tag produces `pkgtui_0.1.1_amd64.deb`, not `pkgtui_v0.1.1_amd64.deb`)
— check the exact asset names on the [latest
release](https://github.com/padovanl/pkgtui/releases/latest) rather than
guessing. The commands use `-f` so curl fails loudly on a wrong URL instead
of silently saving GitHub's "not found" page as if it were the package.

### `.deb` package (Debian/Ubuntu and derivatives)

```bash
curl -fLO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<version>_amd64.deb
sudo apt install ./pkgtui_<version>_amd64.deb
```

### `.snap` package (side-load, no Snap Store)

`pkgtui` needs to invoke `apt`/`snap` on the host system, so the package
uses `classic` confinement:

```bash
curl -fLO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<version>_amd64.snap
sudo snap install --dangerous --classic pkgtui_<version>_amd64.snap
```

### Standalone binary (any Linux distro with apt and/or snap)

```bash
curl -fLO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<version>_linux_amd64.tar.gz
tar -xzf pkgtui_<version>_linux_amd64.tar.gz
sudo mv pkgtui /usr/local/bin/
```

### From source

```bash
git clone https://github.com/padovanl/pkgtui.git
cd pkgtui
go build -o pkgtui .
sudo mv pkgtui /usr/local/bin/
```

## ⌨️ Usage

```bash
pkgtui
```

| Key         | Action                                     |
| ----------- | ------------------------------------------- |
| `←` / `→`   | Switch backend (apt / snap)                 |
| `tab`       | Switch view (Installed / Upgradable / Orphaned\* / Search) |
| `/`         | Search the full apt/snap catalog (then `enter` to run it) |
| `f`         | Filter the packages currently shown, live as you type (`enter` to keep it, `esc` to close without filtering) |
| `↑`/`↓`, `j`/`k` | Navigate the list (mouse wheel and clicks work too) |
| `enter`     | Show details for the selected package       |
| `space`     | Tag/untag the selected package for a batch action |
| `i`         | Install the selected package, or all tagged packages |
| `d`         | Remove the selected package, or all tagged packages |
| `u`         | Upgrade the selected package                |
| `U`         | Upgrade **all** packages (shows what will change first) |
| `S`         | Sort the current view by installed size     |
| `c`         | Cycle the install channel (while confirming a snap install) |
| `H`         | Hold/unhold the selected package (apt)      |
| `C`         | View the selected package's changelog (apt) |
| `P`         | Manage third-party repositories / PPAs (apt) |
| `s`         | Sync the cache (`apt-get update`; no-op on snap) |
| `K`         | Disk cleanup explorer: old kernels, leftover configs, disabled snap revisions |
| `W`         | Why is the selected package installed (manual vs. dependency, reverse-dep tree) |
| `A`         | Unattended-upgrades status (apt)            |
| `O`         | apt+snap overlap view: duplicate installs, stale snaps |
| `V`         | Install a specific version of the selected package / downgrade (apt); revert to the previous revision (snap) |
| `M`         | Metrics dashboard: installed packages ranked by disk usage |
| `X`         | Upgrade conflicts: packages a plain upgrade would keep back (apt) |
| `L`         | Action log: what's run this session, and whether it succeeded |
| `y` / `n`   | Confirm / cancel an action                  |
| `esc`       | Go back                                     |
| `,`         | Open settings (theme, keybindings)          |
| `?`         | Toggle the in-app help screen (full keybinding list) |
| `ctrl+l`    | Force a full screen redraw (fixes a stale/glitched display, same as in vim/bash/htop) |
| `q`         | Quit                                        |

\* Orphaned only appears for apt: packages `apt-get autoremove` would clean up.
All key bindings above except navigation and the y/n/esc confirm keys can be
remapped from the settings screen (`,`).

Actions that change the system (install, remove, upgrade) run with `sudo`
(skipped automatically if you're already root, e.g. inside a container)
attached to a real pseudo-terminal, shown live in a box inside pkgtui.
Keystrokes go straight to the command, so the password prompt and any
interactive dpkg/debconf dialog work exactly as they would from the
command line — `ctrl+c` interrupts the running command rather than
pkgtui itself. The view stays open after the command finishes so you can
read the result; press any key to return.

Every package row starts with a status symbol, also shown as a legend right
under the header and in the `?` help screen:

| Symbol      | Meaning           |
| :---------: | ----------------- |
| `●`         | Installed          |
| `▲`         | Installed, upgrade available |
| `▲` (red)   | Installed, security update available |
| `○`         | Not installed      |
| `◆`         | Held: upgrades blocked (`H` to toggle) |

## ⚙️ Configuration

Settings (theme, rebound keys) and the last-used view live in
`~/.config/pkgtui/config.json`, written by the settings screen (`,`) — there's
normally no need to edit it by hand, but it's plain JSON if you want to.

## ✅ Requirements

- Linux with `apt`/`dpkg` and/or `snapd` installed (having just one of the
  two is fine: the other tab simply reports "not available").
- `sudo` configured for the current user, for privileged operations.
- Go 1.24+ only if building from source (fetching Go modules needs
  internet; the compiled binary itself doesn't).
- Internet access for anything that reaches an actual package repository
  or catalog — search, install, upgrade, changelog. Browsing what's
  already installed or upgradable works fully offline. pkgtui makes no
  network calls of its own; this is exactly the same requirement `apt`
  and `snap` have on their own.
- A terminal that reports 256-color or truecolor support. A bare
  `TERM=xterm` (no `-256color` suffix) gets detected as a 16-color
  terminal, which downsamples every theme's colors to the nearest basic
  ANSI color and can make some of them hard to tell apart. Minimal
  Docker/SSH sessions are the usual culprit — `export TERM=xterm-256color`
  (or `COLORTERM=truecolor` if the real terminal supports it) fixes it.

## 🤝 Contributing

Pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for the
branch policy, local setup, code style, and the release process. Short
version: open PRs against `develop`, not `main`, and run
`gofmt -l . && go vet ./... && go test ./...` before pushing.

## 📄 License

MIT — see [LICENSE](LICENSE).
