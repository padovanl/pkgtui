//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestDiskScreenOpensAndCloses covers "K": the disk cleanup explorer opens
// (real dpkg-query/uname output on whatever machine runs this test) and esc
// returns to the list.
func TestDiskScreenOpensAndCloses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("K")
	h.waitFor("Disk cleanup", 5*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("Disk cleanup", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
}

// TestProvenanceScreenOpensAndCloses covers "W" on the first installed
// package: the "why is this installed" screen opens and esc returns to the
// list.
func TestProvenanceScreenOpensAndCloses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendKey("down") // land on a real row, not just whatever the cursor starts on
	h.sendText("W")
	h.waitFor("Why is", 5*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("Why is", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
}

// TestUnattendedScreenOpensAndCloses covers "A": the unattended-upgrades
// status dashboard opens and esc returns to the list.
func TestUnattendedScreenOpensAndCloses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("A")
	h.waitFor("Unattended upgrades", 5*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("Unattended upgrades", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
}

// TestVersionPickerOpensAndCloses covers "V" on the apt tab: the version
// picker opens (real apt-cache madison output on whatever machine runs
// this test) and esc returns to the list.
func TestVersionPickerOpensAndCloses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendKey("down") // land on a real row, not just whatever the cursor starts on
	h.sendText("V")
	h.waitFor("Install a specific version", 5*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("Install a specific version", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
}

// TestRevertAsksConfirmation covers "V" on the snap tab: since snap has no
// version picker (only revert), it should go straight to a confirmation
// instead of any intermediate screen.
func TestRevertAsksConfirmation(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendKey("right") // switch to the snap tab
	h.waitFor("SNAP —", 5*time.Second)
	h.waitForAbsent("loading...", 20*time.Second)
	if strings.Contains(h.currentFrame(), "No items.") {
		// Caught live: a CI runner with no snaps installed (not even
		// core/snapd itself) has nothing to select, so "V" has nothing to
		// act on and this path can't be exercised — not a pkgtui bug, just
		// an environment this test's premise doesn't hold in.
		t.Skip("no snaps installed on this machine, nothing to revert")
	}
	h.sendKey("down")
	h.sendText("V")
	h.waitFor("Revert", 5*time.Second)

	h.sendText("n") // cancel, don't actually run it
	h.waitForAbsent("Revert", 3*time.Second)
}

// TestOverlapScreenOpensAndCloses covers "O": the app-wide apt+snap overlap
// view opens (spanning both panels, not just the active one) and esc
// returns to whichever panel was active.
func TestOverlapScreenOpensAndCloses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("O")
	h.waitFor("overlap & staleness", 5*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("overlap & staleness", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
}
