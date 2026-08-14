//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestSettingsRebindDisplaysNewKeyImmediately reproduces, byte-for-byte
// through a real pty, the flow reported live: open settings, rebind the
// action that opens settings itself away from its default, and check
// whether the settings screen's own row actually shows the new key —
// both right after rebinding and again after closing and reopening the
// screen with the new key. internal/ui's own tests call Panel.handleKey
// directly and never exercise bubbletea's real render/redraw path, so
// they can't see a bug that only exists in what's actually drawn to the
// terminal — this can.
func TestSettingsRebindDisplaysNewKeyImmediately(t *testing.T) {
	h := newHarness(t)

	// termenv queries the terminal for its background color (OSC 11) on
	// startup and waits briefly for a reply before falling back to
	// defaults; a real terminal answers near-instantly, but this harness
	// doesn't emulate one, so that wait is real here — hence the
	// generous timeout on the very first thing we wait for.
	h.waitFor("pkgtui", 10*time.Second)

	h.sendText(",")
	h.waitFor("pkgtui — settings", 3*time.Second)

	// "Open settings" is the 18th rebindable row (index 17 in
	// internal/ui/keys.go's rebindableKeys, 1-indexed here since row 0 is
	// the theme row) — hardcoded rather than computed, since this test
	// can't import that unexported list; if it ever drifts, the line
	// assertions below fail loudly (wrong row highlighted) rather than
	// silently passing on the wrong row.
	for i := 0; i < 18; i++ {
		h.sendKey("down")
	}
	h.waitForRowValue("Open settings", ",", 2*time.Second)

	h.sendKey("enter") // start capture
	h.waitFor("press a key to rebind", 2*time.Second)
	h.sendText("p") // rebind to p

	line := h.waitForRowValue("Open settings", "p", 2*time.Second)
	if value := afterLabel(line, "Open settings"); strings.Contains(value, ",") {
		t.Fatalf(`right after rebinding: "Open settings" row value = %q, still shows the old ","`, value)
	}

	h.sendKey("esc") // close settings
	h.waitForAbsent("pkgtui — settings", 3*time.Second)

	// The old key must no longer do anything.
	h.sendText(",")
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(h.currentFrame(), "pkgtui — settings") {
		t.Fatal("the old \",\" key still opened settings after rebinding it away")
	}

	h.sendText("p") // reopen with the new key
	h.waitFor("pkgtui — settings", 3*time.Second)

	for i := 0; i < 18; i++ {
		h.sendKey("down")
	}
	line = h.waitForRowValue("Open settings", "p", 2*time.Second)
	if value := afterLabel(line, "Open settings"); strings.Contains(value, ",") {
		t.Fatalf(`after closing and reopening settings: "Open settings" row value = %q, want it to still show "p"`, value)
	}
}

func afterLabel(line, label string) string {
	idx := strings.Index(line, label)
	if idx == -1 {
		return line
	}
	return line[idx+len(label):]
}
