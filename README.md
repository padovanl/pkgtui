# pkgtui 📦

[![CI](https://github.com/padovanl/pkgtui/actions/workflows/ci.yml/badge.svg)](https://github.com/padovanl/pkgtui/actions/workflows/ci.yml)
[![Release](https://github.com/padovanl/pkgtui/actions/workflows/release.yml/badge.svg)](https://github.com/padovanl/pkgtui/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/padovanl/pkgtui?sort=semver)](https://github.com/padovanl/pkgtui/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/padovanl/pkgtui/total)](https://github.com/padovanl/pkgtui/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/padovanl/pkgtui)](go.mod)
[![License: MIT](https://img.shields.io/github/license/padovanl/pkgtui)](LICENSE)

An **htop-style** terminal UI (TUI) to search, install, remove and upgrade
**apt** and **snap** packages, from a single dashboard, without having to
remember the syntax of either package manager.

![pkgtui apt demo](assets/demo.gif)
*Searching, inspecting and installing an **apt** package, live.*

![pkgtui snap demo](assets/demo-snap.gif)
*Same flow on **snap**, including the channel picker (`c` to cycle stable/candidate/beta/edge).*

## ✨ Features

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
- **Multi-select**: tag several packages and install/remove them in one
  batch action.
- **Channel picker**: install a snap from `stable`, `candidate`, `beta` or
  `edge` instead of always defaulting to stable.
- **Hold/pin (apt)**: block a package from being touched by upgrades
  (`apt-mark hold`), with held packages marked `[held]` in the list.
- **Security updates called out**: upgradable packages coming from a
  `-security` repo get a distinct red marker, and the upgrade-all
  confirmation counts them separately.
- **Upgrade all, with a preview**: `U` fetches and shows exactly which
  packages will change before you confirm, instead of a blind "upgrade
  everything?".
- **Changelog viewer (apt)**: see what actually changed in a package
  before upgrading it (`apt-get changelog`).
- **PPA management (apt)**: list, add and remove third-party repositories
  from inside the TUI, gated behind an explicit warning since a bad PPA
  can break `apt update` for the whole system.
- **Settings screen**: cycle between built-in color themes and rebind any
  action key, live, from inside the app — no config file editing required
  (though it's saved to one).
- **Remembers where you left off**: backend and view are restored on the
  next launch.
- **Install/remove/upgrade with confirmation**: every privileged action
  asks for confirmation (`y`/`n`) before running.
- **Live output, without losing the app**: `apt-get`/`snap` commands run
  in a real pseudo-terminal shown in a bordered box inside pkgtui — you
  still get the genuine `sudo` password prompt and any interactive
  dpkg/debconf dialog, but the screen doesn't get handed over. The view
  stays up after the command finishes (green border on success, red on
  failure) until you press a key, so you can actually read the result.
- **Mouse support**: click a tab to switch backend, click a row to select
  it, scroll to navigate.

## 📥 Installation

No verification/store required: grab the asset from the [releases
page](https://github.com/padovanl/pkgtui/releases) and install it locally.

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
| `f`         | Filter the packages currently shown, live as you type |
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
| `y` / `n`   | Confirm / cancel an action                  |
| `esc`       | Go back                                     |
| `,`         | Open settings (theme, keybindings)          |
| `?`         | Toggle the in-app help screen (full keybinding list) |
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
- Go 1.24+ only if building from source.

## 🤝 Contributing

Pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for the
branch policy, local setup, code style, and the release process. Short
version: open PRs against `develop`, not `main`, and run
`gofmt -l . && go vet ./... && go test ./...` before pushing.

## 📄 License

MIT — see [LICENSE](LICENSE).
