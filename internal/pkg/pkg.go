// Package pkg defines the shared types and interface implemented by each
// package manager backend (apt, snap), so the UI layer can treat them
// uniformly.
package pkg

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
