//go:build e2e

// Package e2e drives the real, compiled pkgtui binary through a real
// pseudo-terminal — the same relationship a real terminal emulator has to
// any program running inside it — and asserts on the actual rendered
// output. Unlike internal/ui's tests, which call Panel methods directly
// and never touch bubbletea's render pipeline, this catches bugs that
// only exist in what actually gets drawn to the screen.
//
// Output is fed into a real VT100/xterm emulator (hinshun/vt10x), not
// reconstructed by hand: bubbletea redraws incrementally (cursor moves,
// partial line rewrites, scrolling), and a hand-rolled "split on the last
// cursor-home sequence" heuristic can't reliably tell a genuinely stale
// frame from a fresh incremental update — it was tried here first and
// produced exactly that kind of false read. A real emulator applies each
// escape sequence the way an actual terminal would and always has the
// correct current screen content to query.
//
// Excluded from `go test ./...` (see the build tag above) since it
// compiles a fresh binary and needs a real pty; CI runs it as an
// explicit, separate step. Run locally with: go test -tags e2e ./e2e/...
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

const (
	termCols = 100
	termRows = 34
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pkgtui-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdtemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "pkgtui")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build ../. failed: %v\n%s\n", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// harness runs one pkgtui process attached to a real pty, feeding its
// output into a real terminal emulator, with its own isolated config
// directory so a test never reads or writes the machine's real
// ~/.config/pkgtui.
type harness struct {
	t    *testing.T
	ptmx *os.File
	cmd  *exec.Cmd
	term vt10x.Terminal
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	configDir := t.TempDir()

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+configDir,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: termCols, Rows: termRows})
	if err != nil {
		t.Fatalf("pty.StartWithSize: %v", err)
	}

	h := &harness{t: t, ptmx: ptmx, cmd: cmd, term: vt10x.New(vt10x.WithSize(termCols, termRows))}
	go h.readLoop()

	t.Cleanup(func() {
		_ = h.ptmx.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	return h
}

func (h *harness) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := h.ptmx.Read(buf)
		if n > 0 {
			_, _ = h.term.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// waitReady waits past both startup delays that matter here: termenv's
// OSC 11 background-color query (see newHarness) and the Installed list's
// own background load, which starts empty ("APT — Installed (0)", spinner
// ticking) and only fills in once the dpkg query underneath it returns. A
// test that sends input right after seeing a bullet character can still
// race this: the legend line ("● installed   ▲ upgrade available ...")
// renders immediately as static chrome, bullet included, well before any
// actual package row exists — matching on "● " is matching the legend,
// not proof of loaded content, and input sent while still loading is
// silently dropped rather than queued. Waiting for the loading spinner's
// own text to disappear is unambiguous: it's only ever shown while
// p.loading is true.
func (h *harness) waitReady() {
	h.t.Helper()
	h.waitFor("pkgtui", 10*time.Second)
	// 5s cut it close even locally (some runs used nearly all of it) and
	// wasn't enough at all on a real CI runner, where the initial dpkg
	// query apparently takes longer — 20s gives real headroom instead of
	// shaving margin off an already-tight wait.
	h.waitForAbsent("loading...", 20*time.Second)
}

// currentFrame is the emulated terminal's current screen content, one
// line per row, trailing padding included (the caller strips whatever it
// needs).
func (h *harness) currentFrame() string {
	return h.term.String()
}

// lineContaining returns the current frame's line containing sub, or ""
// if no line matches — for assertions on one specific row rather than
// the whole screen, which is both more precise and more resilient to
// unrelated layout changes elsewhere on screen.
func (h *harness) lineContaining(sub string) string {
	for _, line := range strings.Split(h.currentFrame(), "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

// waitFor polls until the current frame contains sub, failing the test if
// it never does within timeout.
func (h *harness) waitFor(sub string, timeout time.Duration) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(h.currentFrame(), sub) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("timed out after %s waiting for %q in output; last frame:\n%s", timeout, sub, h.currentFrame())
}

// waitForAbsent polls until the current frame no longer contains sub,
// failing the test if it still does after timeout.
func (h *harness) waitForAbsent(sub string, timeout time.Duration) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !strings.Contains(h.currentFrame(), sub) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("timed out after %s waiting for %q to disappear from output; last frame:\n%s", timeout, sub, h.currentFrame())
}

// waitForAny polls until the current frame contains at least one of subs,
// returning which one matched first. Used where an outcome is one of
// several legitimate results (e.g. a live command finishing "done" or
// "failed") rather than one specific string.
func (h *harness) waitForAny(subs []string, timeout time.Duration) string {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		frame := h.currentFrame()
		for _, sub := range subs {
			if strings.Contains(frame, sub) {
				return sub
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("timed out after %s waiting for any of %q in output; last frame:\n%s", timeout, subs, h.currentFrame())
	return ""
}

// waitExit waits for the pkgtui process to exit on its own (e.g. after
// sending "q"), failing the test if it's still running after timeout —
// guards against a hang on quit, which a test that never checks for exit
// would never catch.
func (h *harness) waitExit(timeout time.Duration) {
	h.t.Helper()
	done := make(chan struct{})
	go func() {
		_, _ = h.cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		h.t.Fatalf("process did not exit within %s of quitting", timeout)
	}
}

// waitForLineHavingPrefix polls until some line contains prefix,
// returning that line. Unlike waitForRowValue, this doesn't guard against
// the search string appearing inside a label rather than a value — fine
// for a prefix distinctive enough that it can't coincidentally match
// anything else on screen (e.g. "APT — Installed ("), not for a single
// character that might.
func (h *harness) waitForLineHavingPrefix(prefix string, timeout time.Duration) string {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if line := h.lineContaining(prefix); line != "" {
			return line
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("timed out after %s waiting for a line containing %q; last frame:\n%s", timeout, prefix, h.currentFrame())
	return ""
}

// waitForResultRow polls until a *list row* containing name appears,
// skipping the search box's own echo of the typed query — "🔍 cowsay"
// contains "cowsay" too, and renders above the results, so a plain
// substring search finds it first. Every real result row starts with a
// status symbol (●/▲/○) that the search box line never has.
func (h *harness) waitForResultRow(name string, timeout time.Duration) string {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(h.currentFrame(), "\n") {
			if !strings.Contains(line, name) {
				continue
			}
			if strings.ContainsAny(line, "●▲○") {
				return line
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("timed out after %s waiting for a result row containing %q", timeout, name)
	return ""
}

// waitForRowValue polls until a line starting with rowLabel has expect
// somewhere in the part *after* the label, returning that line. Checking
// the whole line for expect is a trap: with rowLabel "Open settings" and
// expect "p", the label text itself already contains a "p" ("Open"), so
// a whole-line check matches immediately, before the row's actual value
// has updated. Restricting the search to the text after rowLabel avoids
// that and actually waits for the value to change.
func (h *harness) waitForRowValue(rowLabel, expect string, timeout time.Duration) string {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		if line := h.lineContaining(rowLabel); line != "" {
			last = line
			if idx := strings.Index(line, rowLabel); idx != -1 {
				value := line[idx+len(rowLabel):]
				if strings.Contains(value, expect) {
					return line
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("timed out after %s waiting for %q row's value to contain %q; last matching line: %q", timeout, rowLabel, expect, last)
	return ""
}

func (h *harness) sendText(s string) {
	h.t.Helper()
	if _, err := h.ptmx.Write([]byte(s)); err != nil {
		h.t.Fatalf("write %q: %v", s, err)
	}
}

var namedKeys = map[string]string{
	"enter": "\r",
	"esc":   "\x1b",
	"up":    "\x1b[A",
	"down":  "\x1b[B",
	"right": "\x1b[C",
	"left":  "\x1b[D",
}

func (h *harness) sendKey(name string) {
	h.t.Helper()
	b, ok := namedKeys[name]
	if !ok {
		h.t.Fatalf("sendKey: unknown key %q", name)
	}
	h.sendText(b)
}
