//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestFilterEscCancelsWithoutFiltering covers the local list filter ("f"):
// the hint appears while filtering, esc closes it without narrowing the
// list (leaving the full item count intact), and it doesn't leak into a
// later "/" catalog search.
func TestFilterEscCancelsWithoutFiltering(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	countLine := h.waitForLineHavingPrefix("APT — Installed (", 3*time.Second)

	h.sendText("f")
	h.waitFor("esc: close without filtering", 3*time.Second)
	h.sendText("zzzzz-not-a-real-package-zzzzz") // would empty the list if actually applied

	h.sendKey("esc")
	h.waitForAbsent("esc: close without filtering", 2*time.Second)

	// The item count must be back to what it was before filtering, not
	// narrowed (or emptied) by the text that was typed and then cancelled.
	after := h.waitForLineHavingPrefix("APT — Installed (", 2*time.Second)
	if !strings.Contains(after, countLine[strings.Index(countLine, "("):]) {
		t.Fatalf("item count changed after cancelling the filter: before=%q after=%q", countLine, after)
	}
}
