package ui

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestFindDuplicates(t *testing.T) {
	aptPkgs := []pkg.Package{
		{Name: "firefox", Installed: "115.0-1ubuntu1"},
		{Name: "curl", Installed: "7.81.0"},
	}
	snapPkgs := []pkg.Package{
		{Name: "firefox", Installed: "128.0"},
		{Name: "core22", Installed: "20240111"},
	}

	got := findDuplicates(aptPkgs, snapPkgs)
	want := []overlapEntry{{name: "firefox", aptVersion: "115.0-1ubuntu1", snapVersion: "128.0"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findDuplicates() = %#v, want %#v", got, want)
	}
}

func TestFindDuplicatesNoOverlap(t *testing.T) {
	aptPkgs := []pkg.Package{{Name: "curl"}}
	snapPkgs := []pkg.Package{{Name: "core22"}}
	if got := findDuplicates(aptPkgs, snapPkgs); len(got) != 0 {
		t.Errorf("findDuplicates() = %v, want empty", got)
	}
}

// fakeStaler is a minimal pkg.Staler stub for testing findStaleSnaps
// without touching the real filesystem.
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

func TestFindStaleSnaps(t *testing.T) {
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

	got := findStaleSnaps(snapPkgs, staler, now, staleThresholdDays*24*time.Hour)
	want := []staleSnap{{name: "firefox", version: "128.0", lastRefresh: now.Add(-400 * 24 * time.Hour)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findStaleSnaps() = %#v, want %#v", got, want)
	}
}

func TestFindStaleSnapsNilStaler(t *testing.T) {
	if got := findStaleSnaps(nil, nil, time.Now(), staleThresholdDays*24*time.Hour); got != nil {
		t.Errorf("findStaleSnaps() with nil staler = %v, want nil", got)
	}
}
