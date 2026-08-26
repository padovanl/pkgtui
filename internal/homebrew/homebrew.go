// Package homebrew implements pkg.Manager on top of the brew command-line
// tool, covering both formulae and casks (brew install/uninstall resolve
// either kind from a bare name, so the UI doesn't have to care which is
// which).
package homebrew

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/padovanl/pkgtui/internal/pkg"
)

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Name() string { return "brew" }

// command builds a brew invocation that can't kick off an implicit "brew
// update": several read-only subcommands quietly git-fetch first once the
// last fetch is old enough, which would turn merely opening this tab into a
// multi-second network stall nobody asked for. The explicit sync action
// still runs a real "brew update". HOMEBREW_NO_ENV_HINTS keeps brew's
// occasional advice lines out of output meant to be parsed.
func command(args ...string) *exec.Cmd {
	c := exec.Command("brew", args...)
	c.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1")
	return c
}

func (m *Manager) Available() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

// parseListVersionsOutput parses "brew list --versions", one
// "<name> <version>..." line per installed formula or cask. A keg can have
// several versions installed side by side, in which case brew lists them
// all on the same line, oldest first — the last one is what "brew list"
// itself considers current.
func parseListVersionsOutput(out string, pinned map[string]bool) map[string]pkg.Package {
	result := make(map[string]pkg.Package)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		name := f[0]
		result[name] = pkg.Package{
			Name:      name,
			Installed: f[len(f)-1],
			Source:    "brew",
			Status:    pkg.StatusInstalled,
			Held:      pinned[name],
		}
	}
	return result
}

// parseNameListOutput reads the plainest brew output there is: one bare
// name per line, as printed by "brew list --pinned" and "brew uses". Lines
// containing whitespace are brew talking to the user ("Warning: ...", "==>
// Formulae"), never a package name.
func parseNameListOutput(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		names = append(names, name)
	}
	return names
}

func pinnedSet() map[string]bool {
	out, err := command("list", "--pinned").Output()
	if err != nil {
		return map[string]bool{}
	}
	set := map[string]bool{}
	for _, name := range parseNameListOutput(string(out)) {
		set[name] = true
	}
	return set
}

// dirSize sums the sizes of every regular file under dir. brew has no
// equivalent of dpkg's Installed-Size — "brew info --json" doesn't report
// one either — so the only way to rank packages by disk usage in the
// metrics dashboard is to measure the keg itself, the same way snap falls
// back to stat-ing its squashfs image.
func dirSize(dir string) int64 {
	var total int64
	// Errors are deliberately swallowed: a keg being upgraded (or simply
	// unreadable) should report as an unknown size, not fail the listing.
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// brewPath resolves one of brew's own directory queries ("--cellar",
// "--caskroom"), which are plain path lookups and don't touch the network.
func brewPath(flag string) string {
	out, err := command(flag).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Manager) installedMap() (map[string]pkg.Package, error) {
	out, err := command("list", "--versions").Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("brew list --versions: %w", err)
	}
	installed := parseListVersionsOutput(string(out), pinnedSet())

	cellar, caskroom := brewPath("--cellar"), brewPath("--caskroom")
	for name, p := range installed {
		if cellar != "" {
			p.Size = dirSize(filepath.Join(cellar, name))
		}
		if p.Size == 0 && caskroom != "" {
			p.Size = dirSize(filepath.Join(caskroom, name))
		}
		installed[name] = p
	}
	return installed, nil
}

func (m *Manager) ListInstalled() ([]pkg.Package, error) {
	installed, err := m.installedMap()
	if err != nil {
		return nil, err
	}
	outdated, err := outdatedMap()
	if err != nil {
		outdated = map[string]pkg.Package{}
	}
	results := make([]pkg.Package, 0, len(installed))
	for name, p := range installed {
		if _, ok := outdated[name]; ok {
			p.Status = pkg.StatusUpgradable
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// outdatedJSON is the shape of "brew outdated --json=v2" — only the fields
// this needs. JSON rather than the plain listing because brew's own
// human-readable output for this varies with --verbose/--quiet and folds
// casks and formulae into one undifferentiated list of names.
type outdatedJSON struct {
	Formulae []outdatedEntry `json:"formulae"`
	Casks    []outdatedEntry `json:"casks"`
}

type outdatedEntry struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
	Pinned            bool     `json:"pinned"`
}

func parseOutdatedJSON(data string) ([]pkg.Package, error) {
	var parsed outdatedJSON
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return nil, fmt.Errorf("brew outdated --json=v2: %w", err)
	}
	var results []pkg.Package
	for _, e := range append(append([]outdatedEntry{}, parsed.Formulae...), parsed.Casks...) {
		installed := ""
		if len(e.InstalledVersions) > 0 {
			installed = e.InstalledVersions[len(e.InstalledVersions)-1]
		}
		results = append(results, pkg.Package{
			Name:      e.Name,
			Version:   e.CurrentVersion,
			Installed: installed,
			Source:    "brew",
			Status:    pkg.StatusUpgradable,
			Held:      e.Pinned,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

func fetchOutdated() ([]pkg.Package, error) {
	out, err := command("outdated", "--json=v2").Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("brew outdated: %w", err)
	}
	return parseOutdatedJSON(string(out))
}

func outdatedMap() (map[string]pkg.Package, error) {
	pkgs, err := fetchOutdated()
	if err != nil {
		return nil, err
	}
	byName := make(map[string]pkg.Package, len(pkgs))
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	return byName, nil
}

func (m *Manager) ListUpgradable() ([]pkg.Package, error) {
	pkgs, err := fetchOutdated()
	if err != nil {
		return nil, err
	}
	installed, err := m.installedMap()
	if err != nil {
		return pkgs, nil
	}
	for i, p := range pkgs {
		if inst, ok := installed[p.Name]; ok {
			pkgs[i].Size = inst.Size
		}
	}
	return pkgs, nil
}

// parseSearchOutput parses "brew search", which prints one bare name per
// line under "==> Formulae" / "==> Casks" section headers. Neither the
// names nor the search index carry a description, so results only get a
// summary saying which of the two kinds they are — "brew desc" would mean
// one extra process per hit.
func parseSearchOutput(out string, installed map[string]pkg.Package, outdated map[string]pkg.Package) []pkg.Package {
	var results []pkg.Package
	kind := ""
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "==>") {
			switch strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "==>"))) {
			case "formulae":
				kind = "formula"
			case "casks":
				kind = "cask"
			default:
				kind = ""
			}
			continue
		}
		if trimmed == "" || strings.ContainsAny(trimmed, " \t") {
			continue
		}
		p := pkg.Package{
			Name:    trimmed,
			Summary: kind,
			Source:  "brew",
			Status:  pkg.StatusAvailable,
		}
		if inst, ok := installed[trimmed]; ok {
			p.Installed = inst.Installed
			p.Size = inst.Size
			p.Held = inst.Held
			p.Status = pkg.StatusInstalled
		}
		if up, ok := outdated[trimmed]; ok {
			p.Version = up.Version
			p.Status = pkg.StatusUpgradable
		}
		results = append(results, p)
	}
	return results
}

func (m *Manager) Search(query string) ([]pkg.Package, error) {
	// CombinedOutput: brew exits non-zero and explains itself on stderr
	// when a search comes up empty, which is a legitimate result rather
	// than an error worth an error banner.
	out, err := command("search", query).CombinedOutput()
	installed, instErr := m.installedMap()
	if instErr != nil {
		installed = map[string]pkg.Package{}
	}
	outdated, outErr := outdatedMap()
	if outErr != nil {
		outdated = map[string]pkg.Package{}
	}
	results := parseSearchOutput(string(out), installed, outdated)
	if err != nil && len(results) == 0 {
		if strings.Contains(string(out), "No formulae or casks found") {
			return nil, nil
		}
		return nil, fmt.Errorf("brew search: %w", err)
	}
	return results, nil
}

func (m *Manager) Info(name string) (string, error) {
	out, err := command("info", name).Output()
	if err != nil {
		return "", fmt.Errorf("brew info %s: %w", name, err)
	}
	return string(out), nil
}

// None of these go through pkg.MaybeSudo: Homebrew refuses outright to run
// as root ("Running Homebrew as root is extremely dangerous and no longer
// supported"), and its prefix is owned by the calling user anyway.
func (m *Manager) InstallCmd(name string) []string {
	return []string{"brew", "install", name}
}

func (m *Manager) RemoveCmd(name string) []string {
	return []string{"brew", "uninstall", name}
}

func (m *Manager) UpgradeCmd(name string) []string {
	if name == "" {
		return []string{"brew", "upgrade"}
	}
	return []string{"brew", "upgrade", name}
}

func (m *Manager) UpdateCmd() []string {
	return []string{"brew", "update"}
}

// InstallManyCmd installs several formulae/casks in one invocation.
func (m *Manager) InstallManyCmd(names []string) []string {
	return append([]string{"brew", "install"}, names...)
}

// RemoveManyCmd removes several formulae/casks in one invocation.
func (m *Manager) RemoveManyCmd(names []string) []string {
	return append([]string{"brew", "uninstall"}, names...)
}

// HoldCmd pins a formula at its current version so "brew upgrade" skips it.
// Implements pkg.Holder.
func (m *Manager) HoldCmd(name string) []string {
	return []string{"brew", "pin", name}
}

// UnholdCmd releases a previous pin. Implements pkg.Holder.
func (m *Manager) UnholdCmd(name string) []string {
	return []string{"brew", "unpin", name}
}

// parseAutoremoveOutput extracts the formulae "brew autoremove --dry-run"
// would remove: dependencies nothing installed still needs. brew prints a
// "==> Would autoremove N unneeded formulae:" banner followed by one bare
// name per line.
func parseAutoremoveOutput(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "==>") || strings.ContainsAny(trimmed, " \t") {
			continue
		}
		names = append(names, trimmed)
	}
	return names
}

// ListOrphaned returns installed formulae nothing else depends on any more.
// Implements pkg.OrphanLister.
func (m *Manager) ListOrphaned() ([]pkg.Package, error) {
	out, err := command("autoremove", "--dry-run").CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("brew autoremove --dry-run: %w", err)
	}
	names := parseAutoremoveOutput(string(out))
	if len(names) == 0 {
		return nil, nil
	}
	installed, err := m.installedMap()
	if err != nil {
		installed = map[string]pkg.Package{}
	}
	results := make([]pkg.Package, 0, len(names))
	for _, name := range names {
		p := pkg.Package{Name: name, Source: "brew", Status: pkg.StatusInstalled, Summary: "no longer required by anything else"}
		if inst, ok := installed[name]; ok {
			p.Installed = inst.Installed
			p.Size = inst.Size
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// brewSizeRe matches the sizes brew's own disk_usage_readable prints, with
// no space between number and unit ("1.2GB", "930.1KB", "512B").
var brewSizeRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*([KMGT]?B)`)

var brewSizeUnits = map[string]int64{
	"B":  1,
	"KB": 1 << 10,
	"MB": 1 << 20,
	"GB": 1 << 30,
	"TB": 1 << 40,
}

// parseCleanupOutput reads "brew cleanup --dry-run": one "Would remove:
// <path> (<size>)" line per stale download or superseded keg, and a closing
// "This operation would free approximately <size> of disk space." line. The
// total comes from that closing line rather than from summing the
// individual ones, since brew reports a file count instead of a size for
// some entries.
func parseCleanupOutput(out string) (count int, total int64) {
	const freeMarker = "would free approximately "
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Would remove:") {
			count++
			continue
		}
		idx := strings.Index(line, freeMarker)
		if idx == -1 {
			continue
		}
		if m := brewSizeRe.FindStringSubmatch(line[idx+len(freeMarker):]); m != nil {
			n, err := strconv.ParseFloat(m[1], 64)
			if err == nil {
				total = int64(n * float64(brewSizeUnits[m[2]]))
			}
		}
	}
	return count, total
}

// DiskReport surfaces what "brew cleanup" would reclaim: cached downloads
// and superseded kegs brew keeps around indefinitely unless told otherwise.
// Reported as a single entry rather than one per file — brew's own cleanup
// has no way to act on an individual path, so a per-file listing would be a
// list of rows that can't do anything on their own. Implements
// pkg.DiskAnalyzer.
func (m *Manager) DiskReport() ([]pkg.DiskItem, error) {
	out, err := command("cleanup", "--dry-run").CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil, nil
	}
	count, total := parseCleanupOutput(string(out))
	if count == 0 {
		return nil, nil
	}
	return []pkg.DiskItem{{
		Name:   fmt.Sprintf("Homebrew cache and superseded kegs (%d items)", count),
		Reason: "stale downloads and outdated versions brew keeps until told to clean up",
		Size:   total,
		Argv:   []string{"brew", "cleanup"},
	}}, nil
}

// infoJSON is the slice of "brew info --json=v2 <name>" this needs: whether
// the installed keg was asked for by name or only pulled in as someone
// else's dependency.
type infoJSON struct {
	Formulae []struct {
		Installed []struct {
			InstalledOnRequest bool `json:"installed_on_request"`
		} `json:"installed"`
	} `json:"formulae"`
}

func parseInstalledOnRequest(data string) bool {
	var parsed infoJSON
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return false
	}
	for _, f := range parsed.Formulae {
		for _, i := range f.Installed {
			if i.InstalledOnRequest {
				return true
			}
		}
	}
	return false
}

// Provenance reports whether name was installed on purpose or dragged in as
// a dependency, and which installed formulae still depend on it. Implements
// pkg.ProvenanceProvider.
func (m *Manager) Provenance(name string) (pkg.Provenance, error) {
	manual := false
	if out, err := command("info", "--json=v2", name).Output(); err == nil {
		manual = parseInstalledOnRequest(string(out))
	}
	var revdeps []string
	if out, err := command("uses", "--installed", name).Output(); err == nil {
		revdeps = parseNameListOutput(string(out))
	}
	return pkg.Provenance{Manual: manual, ReverseDeps: revdeps}, nil
}

var _ pkg.Manager = (*Manager)(nil)
var _ pkg.BatchManager = (*Manager)(nil)
var _ pkg.Holder = (*Manager)(nil)
var _ pkg.OrphanLister = (*Manager)(nil)
var _ pkg.DiskAnalyzer = (*Manager)(nil)
var _ pkg.ProvenanceProvider = (*Manager)(nil)
