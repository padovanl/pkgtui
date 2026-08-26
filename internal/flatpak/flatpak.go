// Package flatpak implements pkg.Manager on top of the flatpak
// command-line client.
package flatpak

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/padovanl/pkgtui/internal/pkg"
)

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Name() string { return "flatpak" }

func (m *Manager) Available() bool {
	_, err := exec.LookPath("flatpak")
	return err == nil
}

// command builds a flatpak invocation with a fixed locale: flatpak renders
// sizes through glib's locale-aware formatter, which on e.g. an it_IT
// system prints "1,2 MB" — a decimal comma parseSize would otherwise read
// as a thousands separator, inflating every size a thousandfold.
func command(args ...string) *exec.Cmd {
	c := exec.Command("flatpak", args...)
	c.Env = append(os.Environ(), "LC_ALL=C")
	return c
}

// sizeRe matches the human-readable sizes flatpak's "size" column prints
// ("1.2 MB", "455 kB", "3.0 GiB"), the only form it offers — unlike dpkg's
// Installed-Size there's no machine-readable byte count anywhere.
var sizeRe = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*([kKMGT]i?)?B$`)

var sizeUnits = map[string]int64{
	"":   1,
	"k":  1000,
	"K":  1000,
	"M":  1000 * 1000,
	"G":  1000 * 1000 * 1000,
	"T":  1000 * 1000 * 1000 * 1000,
	"ki": 1 << 10,
	"Ki": 1 << 10,
	"Mi": 1 << 20,
	"Gi": 1 << 30,
	"Ti": 1 << 40,
}

func parseSize(s string) int64 {
	m := sizeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return int64(n * float64(sizeUnits[m[2]]))
}

// columns are tab-separated whenever stdout isn't a terminal, which is
// always the case here (every listing is captured through a pipe), so the
// free-text columns keep any spaces they contain.
func fields(line string) []string { return strings.Split(line, "\t") }

// isHeader spots the column-title row flatpak prints when it thinks it's
// talking to a terminal — it shouldn't appear in piped output, but skipping
// it costs nothing and keeps the parsers usable either way.
func isHeader(f []string) bool {
	return len(f) > 0 && (f[0] == "Application ID" || f[0] == "Name" || f[0] == "ID")
}

// parseListOutput parses:
//
//	flatpak list --app --columns=application,version,name,origin,size
//
// into application ID -> installed package. The application ID is what
// every other flatpak command takes as an argument, so that's the Name the
// UI works with; the human-friendly title goes in the summary instead,
// where it stays searchable without becoming something you can't act on.
func parseListOutput(out string, masked map[string]bool) map[string]pkg.Package {
	result := make(map[string]pkg.Package)
	for _, line := range strings.Split(out, "\n") {
		f := fields(line)
		if len(f) < 5 || isHeader(f) {
			continue
		}
		appID := strings.TrimSpace(f[0])
		if appID == "" {
			continue
		}
		summary := strings.TrimSpace(f[2])
		if origin := strings.TrimSpace(f[3]); origin != "" {
			summary = strings.TrimSpace(summary + " (" + origin + ")")
		}
		result[appID] = pkg.Package{
			Name:      appID,
			Installed: strings.TrimSpace(f[1]),
			Summary:   summary,
			Size:      parseSize(f[4]),
			Source:    "flatpak",
			Status:    pkg.StatusInstalled,
			Held:      masked[appID],
		}
	}
	return result
}

func (m *Manager) installedMap() (map[string]pkg.Package, error) {
	out, err := command("list", "--app", "--columns=application,version,name,origin,size").Output()
	if err != nil {
		// A system with flatpak installed but nothing in it exits non-zero
		// on some versions; no output at all is the only real failure.
		if len(out) == 0 {
			return map[string]pkg.Package{}, nil
		}
	}
	return parseListOutput(string(out), maskedSet()), nil
}

// parseMaskOutput reads "flatpak mask"'s listing of currently masked
// patterns (one per line). Anything containing whitespace is a message
// rather than a pattern — flatpak prints one when nothing is masked at all.
func parseMaskOutput(out string) map[string]bool {
	masked := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		p := strings.TrimSpace(line)
		if p == "" || strings.ContainsAny(p, " \t") {
			continue
		}
		// Patterns may be full refs ("app/org.gimp.GIMP/x86_64/stable") or
		// bare application IDs; key on the ID either way.
		if strings.Contains(p, "/") {
			parts := strings.Split(p, "/")
			if len(parts) > 1 {
				p = parts[1]
			}
		}
		masked[strings.TrimSuffix(p, "*")] = true
	}
	return masked
}

func maskedSet() map[string]bool {
	out, err := command("mask").Output()
	if err != nil {
		return map[string]bool{}
	}
	return parseMaskOutput(string(out))
}

// ListInstalled deliberately doesn't cross-reference ListUpgradable the way
// apt does: apt reads a local cache, while "flatpak remote-ls --updates"
// talks to every configured remote over the network, and the panel already
// fetches it separately for the upgradable view.
func (m *Manager) ListInstalled() ([]pkg.Package, error) {
	installed, err := m.installedMap()
	if err != nil {
		return nil, err
	}
	results := make([]pkg.Package, 0, len(installed))
	for _, p := range installed {
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// parseUpdatesOutput parses:
//
//	flatpak remote-ls --updates --app --columns=application,version,origin
//
// which lists exactly the installed apps a "flatpak update" would touch.
func parseUpdatesOutput(out string, installed map[string]pkg.Package) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		f := fields(line)
		if len(f) < 2 || isHeader(f) {
			continue
		}
		appID := strings.TrimSpace(f[0])
		if appID == "" {
			continue
		}
		p := pkg.Package{
			Name:    appID,
			Version: strings.TrimSpace(f[1]),
			Source:  "flatpak",
			Status:  pkg.StatusUpgradable,
		}
		if inst, ok := installed[appID]; ok {
			p.Installed = inst.Installed
			p.Summary = inst.Summary
			p.Size = inst.Size
			p.Held = inst.Held
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

func (m *Manager) ListUpgradable() ([]pkg.Package, error) {
	out, err := command("remote-ls", "--updates", "--app", "--columns=application,version,origin").Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("flatpak remote-ls --updates: %w", err)
	}
	installed, err := m.installedMap()
	if err != nil {
		installed = map[string]pkg.Package{}
	}
	return parseUpdatesOutput(string(out), installed), nil
}

// parseSearchOutput parses:
//
//	flatpak search --columns=application,version,name,description,remotes
//
// annotating each hit with install/upgrade status from the installed set.
func parseSearchOutput(out string, installed map[string]pkg.Package) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		f := fields(line)
		if len(f) < 4 || isHeader(f) {
			continue
		}
		appID := strings.TrimSpace(f[0])
		if appID == "" {
			continue
		}
		summary := strings.TrimSpace(f[3])
		if title := strings.TrimSpace(f[2]); title != "" {
			summary = title + " — " + summary
		}
		p := pkg.Package{
			Name:    appID,
			Version: strings.TrimSpace(f[1]),
			Summary: summary,
			Source:  "flatpak",
			Status:  pkg.StatusAvailable,
		}
		if inst, ok := installed[appID]; ok {
			p.Installed = inst.Installed
			p.Size = inst.Size
			p.Held = inst.Held
			p.Status = pkg.StatusInstalled
			if inst.Installed != p.Version && p.Version != "" {
				p.Status = pkg.StatusUpgradable
			}
		}
		results = append(results, p)
	}
	return results
}

func (m *Manager) Search(query string) ([]pkg.Package, error) {
	// CombinedOutput, not Output: "No matches found" goes to stderr and
	// makes flatpak exit non-zero, which is a legitimate empty result
	// rather than something to raise an error banner over.
	out, err := command("search", "--columns=application,version,name,description,remotes", query).CombinedOutput()
	installed, instErr := m.installedMap()
	if instErr != nil {
		installed = map[string]pkg.Package{}
	}
	results := parseSearchOutput(string(out), installed)
	if err != nil && len(results) == 0 {
		if strings.Contains(string(out), "No matches found") {
			return nil, nil
		}
		return nil, fmt.Errorf("flatpak search: %w", err)
	}
	return results, nil
}

// remoteInfoTimeout bounds the remote-info fallback in Info, which fetches
// metadata over the network for every configured remote in turn and would
// otherwise hang the UI on a slow or unreachable one.
const remoteInfoTimeout = 15 * time.Second

func remotes() []string {
	out, err := command("remotes", "--columns=name").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(fields(line)[0])
		if name == "" || name == "Name" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// Info returns flatpak's own details block. "flatpak info" only knows about
// installed refs, so for a search hit that isn't installed yet it falls
// back to asking each configured remote about it.
func (m *Manager) Info(name string) (string, error) {
	if out, err := command("info", name).Output(); err == nil {
		return string(out), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteInfoTimeout)
	defer cancel()
	for _, remote := range remotes() {
		c := exec.CommandContext(ctx, "flatpak", "remote-info", remote, name)
		c.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := c.Output(); err == nil {
			return string(out), nil
		}
	}
	return "", fmt.Errorf("flatpak info %s: not installed, and no configured remote has it", name)
}

// The install/remove/update commands deliberately don't go through
// pkg.MaybeSudo: flatpak asks polkit for authorization when it needs it
// (and needs none at all for a --user installation), so prefixing sudo
// would both bypass that and install into root's own user installation on
// systems where that's the default.
func (m *Manager) InstallCmd(name string) []string {
	return []string{"flatpak", "install", "-y", name}
}

func (m *Manager) RemoveCmd(name string) []string {
	return []string{"flatpak", "uninstall", "-y", name}
}

func (m *Manager) UpgradeCmd(name string) []string {
	if name == "" {
		return []string{"flatpak", "update", "-y"}
	}
	return []string{"flatpak", "update", "-y", name}
}

// UpdateCmd refreshes the appstream metadata flatpak search reads, the
// closest thing flatpak has to "apt-get update" (the ref metadata itself is
// always fetched on demand).
func (m *Manager) UpdateCmd() []string {
	return []string{"flatpak", "update", "--appstream"}
}

// InstallManyCmd installs several apps in one invocation.
func (m *Manager) InstallManyCmd(names []string) []string {
	return append([]string{"flatpak", "install", "-y"}, names...)
}

// RemoveManyCmd removes several apps in one invocation.
func (m *Manager) RemoveManyCmd(names []string) []string {
	return append([]string{"flatpak", "uninstall", "-y"}, names...)
}

// HoldCmd masks an app so "flatpak update" skips it. Implements pkg.Holder.
func (m *Manager) HoldCmd(name string) []string {
	return []string{"flatpak", "mask", name}
}

// UnholdCmd lifts a previous mask. Implements pkg.Holder.
func (m *Manager) UnholdCmd(name string) []string {
	return []string{"flatpak", "mask", "--remove", name}
}

var _ pkg.Manager = (*Manager)(nil)
var _ pkg.BatchManager = (*Manager)(nil)
var _ pkg.Holder = (*Manager)(nil)
