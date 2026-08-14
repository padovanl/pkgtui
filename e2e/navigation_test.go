//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestSwitchBackend covers the apt/snap tab switch ("right"/"left"): the
// header updates to the other backend and back.
func TestSwitchBackend(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendKey("right")
	h.waitFor("SNAP —", 5*time.Second)

	h.sendKey("left")
	h.waitFor("APT —", 5*time.Second)
}

// TestHelpScreenOpensAndCloses covers "?": the full keybinding reference
// renders, and esc returns to the list.
func TestHelpScreenOpensAndCloses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("?")
	h.waitFor("pkgtui — help", 3*time.Second)
	h.waitFor("Navigation", 2*time.Second)
	h.waitFor("Actions", 2*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("pkgtui — help", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
}

// TestQuitExitsCleanly covers "q": the process actually exits instead of
// hanging — a real subprocess-level check no internal-state test can do.
func TestQuitExitsCleanly(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("q")
	h.waitExit(5 * time.Second)
}
