package ui

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestFindDuplicates(t *testing.T) {
	byBackend := map[string][]pkg.Package{
		"apt": {
			{Name: "firefox", Installed: "115.0-1ubuntu1"},
			{Name: "curl", Installed: "7.81.0"},
		},
		"snap": {
			{Name: "firefox", Installed: "128.0"},
			{Name: "core22", Installed: "20240111"},
		},
	}

	got := findDuplicates(byBackend, []string{"apt", "snap"})
	want := []overlapEntry{{name: "firefox", installs: []backendVersion{
		{backend: "apt", version: "115.0-1ubuntu1"},
		{backend: "snap", version: "128.0"},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findDuplicates() = %#v, want %#v", got, want)
	}
}

// With more than two backends the same app really can come from three
// places at once (distro package, flatpak, brew), and each of them belongs
// in the entry — in the tab order they're listed in, not map order.
func TestFindDuplicatesAcrossThreeBackends(t *testing.T) {
	byBackend := map[string][]pkg.Package{
		"apt":     {{Name: "gimp", Installed: "2.10.34"}},
		"flatpak": {{Name: "gimp", Installed: "2.10.36"}},
		"brew":    {{Name: "gimp", Installed: "2.10.38"}},
	}

	got := findDuplicates(byBackend, []string{"apt", "flatpak", "brew"})
	want := []overlapEntry{{name: "gimp", installs: []backendVersion{
		{backend: "apt", version: "2.10.34"},
		{backend: "flatpak", version: "2.10.36"},
		{backend: "brew", version: "2.10.38"},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findDuplicates() = %#v, want %#v", got, want)
	}
}

func TestFindDuplicatesNoOverlap(t *testing.T) {
	byBackend := map[string][]pkg.Package{
		"apt":  {{Name: "curl"}},
		"snap": {{Name: "core22"}},
	}
	if got := findDuplicates(byBackend, []string{"apt", "snap"}); len(got) != 0 {
		t.Errorf("findDuplicates() = %v, want empty", got)
	}
}

// fakeStaler is a minimal pkg.Staler stub for testing findStale without
// touching the real filesystem.
type fakeStaler struct {
	revisions map[string]string
	times     map[string]time.Time // keyed "name@revision"
}

func (f fakeStaler) InstalledRevisions() (map[string]string, error) { return f.revisions, nil }

var errNoRefreshTime = errors.New("no refresh time recorded")

func (f fakeStaler) RefreshTime(name, revision string) (time.Time, error) {
	t, ok := f.times[name+"@"+revision]
	if !ok {
		return time.Time{}, errNoRefreshTime
	}
	return t, nil
}

func TestFindStale(t *testing.T) {
	now := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	snapPkgs := []pkg.Package{
		{Name: "firefox", Installed: "128.0"},
		{Name: "fresh-snap", Installed: "1.0"},
	}
	staler := fakeStaler{
		revisions: map[string]string{"firefox": "3212", "fresh-snap": "5"},
		times: map[string]time.Time{
			"firefox@3212": now.Add(-400 * 24 * time.Hour), // clearly stale
			"fresh-snap@5": now.Add(-1 * 24 * time.Hour),   // recently refreshed
		},
	}

	got := findStale("snap", snapPkgs, staler, now, staleThresholdDays*24*time.Hour)
	want := []staleEntry{{backend: "snap", name: "firefox", version: "128.0", lastRefresh: now.Add(-400 * 24 * time.Hour)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findStale() = %#v, want %#v", got, want)
	}
}

func TestFindStaleNilStaler(t *testing.T) {
	if got := findStale("snap", nil, nil, time.Now(), staleThresholdDays*24*time.Hour); got != nil {
		t.Errorf("findStale() with nil staler = %v, want nil", got)
	}
}
