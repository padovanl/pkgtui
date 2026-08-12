# pkgtui

An **htop-style** terminal UI (TUI) to search, install, remove and upgrade
**apt** and **snap** packages, from a single dashboard, without having to
remember the syntax of either package manager.

```
  pkgtui                                                              apt      snap
 APT — Installed (543)
● bash                         5.1-6ubuntu1.1
● curl                         7.81.0-1ubuntu1.25
▲ nftables                     1.0.2-1ubuntu3
● python3                      3.10.6-1~22.04.1
...
  → apt/snap  tab view  / search  enter details  i install  d remove  u upgrade  U upgrade all  s sync  q quit
```

## Features

- **Two separate backends**: dedicated tabs for `apt` and `snap`, each with
  its own state (the two worlds are never mixed together).
- **Live search**: search packages by name/description (`apt-cache search`
  / `snap find`).
- **Installed / Upgradable**: dedicated views to see at a glance what you
  have installed and what has an update available.
- **Package details**: description, version, dependencies (apt) or
  publisher/channels (snap) before you install anything.
- **Install/remove/upgrade with confirmation**: every privileged action
  asks for confirmation (`y`/`n`) before running.
- **Live output**: `apt-get`/`snap` commands run attached to the real
  terminal (including the `sudo` password prompt), so you see exactly
  what's happening, just like from the command line.
- **Upgrade all**: one key to run `apt-get upgrade` or `snap refresh` on
  every package of the active backend.

## Installation

No verification/store required: grab the asset from the [releases
page](https://github.com/padovanl/pkgtui/releases) and install it locally.

### `.deb` package (Debian/Ubuntu and derivatives)

```bash
curl -LO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<version>_amd64.deb
sudo apt install ./pkgtui_<version>_amd64.deb
```

### `.snap` package (side-load, no Snap Store)

`pkgtui` needs to invoke `apt`/`snap` on the host system, so the package
uses `classic` confinement:

```bash
curl -LO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<version>_amd64.snap
sudo snap install --dangerous --classic pkgtui_<version>_amd64.snap
```

### Standalone binary (any Linux distro with apt and/or snap)

```bash
curl -LO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<version>_linux_amd64.tar.gz
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

## Usage

```bash
pkgtui
```

| Key         | Action                                     |
| ----------- | ------------------------------------------- |
| `←` / `→`   | Switch backend (apt / snap)                 |
| `tab`       | Switch view (Installed / Upgradable / Search) |
| `/`         | Search (then `enter` to run the search)     |
| `↑`/`↓`, `j`/`k` | Navigate the list                      |
| `enter`     | Show details for the selected package       |
| `i`         | Install the selected package                |
| `d`         | Remove the selected package                 |
| `u`         | Upgrade the selected package                |
| `U`         | Upgrade **all** packages on the active backend |
| `s`         | Sync the cache (`apt-get update`; no-op on snap) |
| `y` / `n`   | Confirm / cancel an action                  |
| `esc`       | Go back                                     |
| `q`         | Quit                                        |

Actions that change the system (install, remove, upgrade) run with `sudo`:
pkgtui hands control of the terminal to the real command, so you'll see any
password prompt and `apt`/`snap` output exactly as you would from the
command line.

## Requirements

- Linux with `apt`/`dpkg` and/or `snapd` installed (having just one of the
  two is fine: the other tab simply reports "not available").
- `sudo` configured for the current user, for privileged operations.

## Building & packaging (for contributors)

The project uses [goreleaser](https://goreleaser.com) to build multi-arch
binaries and generate the `.deb` (via its built-in nfpm integration), and
[snapcraft](https://snapcraft.io/docs/snapcraft-overview) for the `.snap`
(see `snap/snapcraft.yaml`).

Development happens on `develop`; releases are cut from `main`: merge
`develop` into `main` when it's ready, then tag `main` with `vX.Y.Z` to
trigger the release workflow (which refuses to run on tags that aren't on
`main`).

```bash
# Local build + .deb, without publishing anything
goreleaser release --snapshot --clean --skip=publish

# Local snap package
snapcraft pack
```

Official releases are built automatically by GitHub Actions on every
`vX.Y.Z` tag (see `.github/workflows/release.yml`).

## License

MIT — see [LICENSE](LICENSE).
