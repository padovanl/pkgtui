// Package pkg defines the shared types and interface implemented by each
// package manager backend (apt, snap), so the UI layer can treat them
// uniformly.
package pkg

import (
	"os"
	"time"
)

// geteuid is a variable (not a direct os.Geteuid call) so tests can swap it
// out without needing to actually run as root.
var geteuid = os.Geteuid

// MaybeSudo prefixes argv with "sudo" unless we're already running as root
// (Geteuid() == 0) — common inside containers, which frequently run as
// root and often don't even have a "sudo" binary installed, so always
// prepending it would make every privileged action fail with a plain
// "command not found" instead of just... not needing it.
func MaybeSudo(argv []string) []string {
	if geteuid() == 0 {
		return argv
	}
	return append([]string{"sudo"}, argv...)
}

// Status describes a package's relationship to the local system.
type Status int

const (
	StatusAvailable Status = iota
	StatusInstalled
	StatusUpgradable
)

// Package is a single result from a search or listing operation.
type Package struct {
	Name      string
	Version   string // candidate/available version
	Installed string // installed version, if any
	Summary   string
	Size      int64 // installed size in bytes, 0 if unknown
	Status    Status
	Source    string // "apt" or "snap"
	Held      bool   // upgrades blocked (apt-mark hold / snap refresh --hold)
	Security  bool   // upgrade comes from a security repository/origin
}

// Manager is implemented by each backend (apt, snap).
type Manager interface {
	// Name returns the backend identifier ("apt" or "snap").
	Name() string

	// Available reports whether the underlying tool exists on this system.
	Available() bool

	// Search looks up packages matching query.
	Search(query string) ([]Package, error)

	// ListInstalled returns all currently installed packages.
	ListInstalled() ([]Package, error)

	// ListUpgradable returns installed packages that have an upgrade available.
	ListUpgradable() ([]Package, error)

	// Info returns a human-readable details block for a package.
	Info(name string) (string, error)

	// InstallCmd returns the argv for installing name, meant to be run
	// interactively (e.g. via an attached TTY) since it may require a
	// password and prints live progress.
	InstallCmd(name string) []string

	// RemoveCmd returns the argv for removing name.
	RemoveCmd(name string) []string

	// UpgradeCmd returns the argv to upgrade a single package, or all
	// packages when name is empty.
	UpgradeCmd(name string) []string

	// UpdateCmd returns the argv to refresh the package index/cache, or
	// nil if the backend has no such concept.
	UpdateCmd() []string
}

// OrphanLister is implemented by backends that can report packages no
// longer required by anything else (apt's autoremove candidates). The UI
// only shows the "Orphaned" view for backends implementing this.
type OrphanLister interface {
	ListOrphaned() ([]Package, error)
}

// BatchManager is implemented by backends that can install/remove several
// packages in a single invocation, for the UI's multi-select action.
type BatchManager interface {
	InstallManyCmd(names []string) []string
	RemoveManyCmd(names []string) []string
}

// ChannelInstaller is implemented by backends with a channel/track concept
// for installs (snap's stable/candidate/beta/edge). The UI only offers a
// channel picker for backends implementing this.
type ChannelInstaller interface {
	Channels() []string
	InstallChannelCmd(name, channel string) []string
}

// Holder is implemented by backends that can pin a package so it's skipped
// by upgrades (apt-mark hold). Scoped to apt: snap can hold too, but has no
// reliable way to report which snaps currently are, which would leave the
// UI showing a toggle it can't ever show the true state of.
type Holder interface {
	HoldCmd(name string) []string
	UnholdCmd(name string) []string
}

// Changelogger is implemented by backends that can fetch a package's
// changelog (apt-get changelog). Network access required; scoped to apt,
// which has no snap equivalent.
type Changelogger interface {
	Changelog(name string) (string, error)
}

// PPA describes one third-party APT source.
type PPA struct {
	Name        string // e.g. "ppa:someone/something"
	Description string
}

// PPAManager is implemented by backends with a concept of addable
// third-party repositories (apt's PPAs via add-apt-repository). Scoped to
// apt; snap has no equivalent.
type PPAManager interface {
	ListPPAs() ([]PPA, error)
	AddPPACmd(ppa string) []string
	RemovePPACmd(ppa PPA) []string
}

// DiskItem is one reclaimable-space finding surfaced by the disk explorer:
// something taking up real disk space without adding value any more (an old
// kernel, a leftover config from a removed package, a disabled snap
// revision), distinct from the "still useful but unused" packages already
// covered by OrphanLister.
type DiskItem struct {
	Name   string
	Reason string // e.g. "old kernel", "leftover config", "disabled revision"
	Size   int64  // bytes, 0 if unknown
	Argv   []string
}

// DiskAnalyzer is implemented by backends that can point out installed
// things taking up disk space that almost nobody actually wants kept
// around: old kernels and orphaned config files left behind by a removed
// package (apt), or disabled old snap revisions kept as a rollback safety
// net (snap).
type DiskAnalyzer interface {
	DiskReport() ([]DiskItem, error)
}

// Provenance describes why a package is present on the system: whether it
// was explicitly asked for or only pulled in as someone else's dependency,
// and what (if anything) currently depends on it.
type Provenance struct {
	Manual      bool // explicitly installed (apt-mark manual), not just a dependency
	ReverseDeps []string
}

// ProvenanceProvider is implemented by backends that can explain why a
// package is installed. Scoped to apt (apt-mark + apt-cache rdepends);
// snap's flat, mostly-dependency-free package model has no real equivalent.
type ProvenanceProvider interface {
	Provenance(name string) (Provenance, error)
}

// UnattendedUpgradesStatus summarizes silent background upgrades, which
// otherwise leave no trace anywhere the user is likely to look.
type UnattendedUpgradesStatus struct {
	Enabled      bool
	LastRunTime  string   // human-readable, "" if unknown/never
	LastPackages []string // packages touched by the most recent run
	NextRunTime  string   // human-readable estimate, "" if unknown
}

// UnattendedUpgradesReporter is implemented by backends with a silent
// background-upgrade mechanism worth surfacing. Scoped to apt
// (unattended-upgrades); snap's auto-refresh has no comparable per-run log.
type UnattendedUpgradesReporter interface {
	UnattendedUpgradesStatus() (UnattendedUpgradesStatus, error)
}

// PackageVersion is one selectable version of a package, as reported by
// VersionLister.
type PackageVersion struct {
	Version string
	Origin  string // repo/origin this version comes from, "" if unknown
	Current bool   // this is the currently installed version
}

// VersionLister is implemented by backends that can list every version of a
// package actually available to install, not just the single "candidate"
// version normally offered — so a user can deliberately install an older
// one, or downgrade after a bad upgrade, without hand-typing
// `apt-get install pkg=version` argv syntax from memory. Scoped to apt
// (apt-cache madison); snap has no comparable "pick an exact version"
// concept, only revisions (see Reverter).
type VersionLister interface {
	AvailableVersions(name string) ([]PackageVersion, error)
	InstallVersionCmd(name, version string) []string
}

// Reverter is implemented by backends with a built-in "go back to what was
// there before" mechanism distinct from picking an explicit version (snap's
// `snap revert`, which restores the previous revision's data/config along
// with its binary, not just the binary — snap keeps the previous revision
// around specifically for this). Scoped to snap.
type Reverter interface {
	RevertCmd(name string) []string
}

// UpgradeConflict is a package whose upgrade is blocked by something other
// than an explicit hold: typically a dependency change (a new or removed
// package) a conservative upgrade won't perform on its own.
type UpgradeConflict struct {
	Name   string
	Reason string
}

// ConflictReporter is implemented by backends that can detect upgrades
// blocked by dependency requirements — distinct from an explicit hold,
// which the UI already surfaces elsewhere (the ◆ marker). Scoped to apt:
// "apt-get upgrade" (not dist-upgrade) reports exactly this as packages it
// "kept back"; snap's flat dependency model has no comparable concept.
type ConflictReporter interface {
	UpgradeConflicts() ([]UpgradeConflict, error)
}

// Staler is implemented by backends that can report when a package's
// currently installed version was actually last refreshed, so the UI can
// flag ones that have sat untouched for a long time — apt already surfaces
// this indirectly (the ▲ upgradable marker means a newer version exists),
// but snap has nothing equivalent: a snap can go untouched for years
// without ever showing as "behind" if nothing newer happens to exist on its
// tracked channel, or the machine simply never runs `snap refresh`. Scoped
// to snap.
type Staler interface {
	// InstalledRevisions maps each installed package's name to an opaque
	// revision identifier the backend can later resolve a refresh time for.
	InstalledRevisions() (map[string]string, error)
	// RefreshTime returns when name's given revision was actually applied.
	RefreshTime(name, revision string) (time.Time, error)
}
