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
	Version   string   // candidate/available version
	Installed string   // installed version, if any
	Summary   string
	Status    Status
	Source    string // "apt" or "snap"
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
