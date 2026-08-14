//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestThemePickerCyclesLive covers the settings screen's theme row:
// left/right browse themes with the change applied immediately (visible
// in the row's own "name (n/total)" position indicator), and browsing
// left past the first entry wraps to the last one instead of getting
// stuck or going out of range.
func TestThemePickerCyclesLive(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText(",")
	h.waitFor("pkgtui — settings", 3*time.Second)
	h.waitForRowValue("Theme:", "1/", 2*time.Second)

	h.sendKey("right")
	h.waitForRowValue("Theme:", "2/", 2*time.Second)

	h.sendKey("right")
	h.waitForRowValue("Theme:", "3/", 2*time.Second)

	h.sendKey("left")
	h.waitForRowValue("Theme:", "2/", 2*time.Second)

	h.sendKey("left")
	h.waitForRowValue("Theme:", "1/", 2*time.Second)

	// Wrapping left from the first theme (1/11) must land on the last one
	// (11/11), not get stuck at 1/11 or go out of range. 11 is the current
	// built-in theme count (internal/ui/styles.go's ThemeNames) — update
	// this if that count changes.
	h.sendKey("left")
	h.waitForRowValue("Theme:", "11/11", 2*time.Second)
}
