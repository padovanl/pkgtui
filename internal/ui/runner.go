package ui

import (
	"os"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
)

// termBuffer turns raw pty output into text fit for a scrolling viewport:
// it collapses '\r'-based progress-bar overwrites (e.g. "Reading package
// lists... 45%\rReading package lists... 89%") into a single evolving
// line instead of spamming one line per update, and strips ANSI escape
// sequences we have no terminal to actually interpret.
type termBuffer struct {
	lines   []string
	current strings.Builder
	// pendingCR is true when the previous Write() call ended in a '\r'
	// whose following byte we hadn't seen yet, so we couldn't tell if it
	// was a CRLF line ending or a standalone progress-bar-style reset.
	pendingCR bool
}

// ansiEscapeRe matches CSI ("\x1b[...letter"), OSC ("\x1b]...BEL"), and the
// handful of other short escape forms apt/dpkg/dpkg-progress commonly
// emit. It's applied per chunk, so a sequence split exactly across two
// Read() calls can leave a stray fragment visible for one refresh; given
// how short these sequences are relative to the 4KB read buffer, that's a
// rare cosmetic blip rather than something worth a stateful parser for.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[()][A-Za-z0-9]|\x1b[=>ME7 -/]`)

// Write processes one chunk of raw pty output. A pty in normal (cooked)
// mode rewrites every outgoing '\n' to "\r\n" (the ONLCR termios flag), so
// a '\r' immediately followed by '\n' is just an ordinary line ending, not
// a request to erase the line — only a '\r' that ISN'T followed by '\n' is
// a genuine progress-bar-style "return to column 0 and overwrite".
// Getting this wrong (treating every trailing \r as an erase) silently
// blanks every single line of output, since dpkg/apt always end lines
// with the standard CRLF pair.
func (t *termBuffer) Write(p []byte) {
	clean := ansiEscapeRe.ReplaceAll(p, nil)
	for i := 0; i < len(clean); i++ {
		b := clean[i]
		if t.pendingCR {
			t.pendingCR = false
			if b == '\n' {
				t.lines = append(t.lines, t.current.String())
				t.current.Reset()
				continue
			}
			t.current.Reset() // the pending \r was a standalone reset after all
		}
		switch b {
		case '\r':
			if i+1 >= len(clean) {
				t.pendingCR = true // resolve once we see what follows
			} else if clean[i+1] == '\n' {
				// leave it for the '\n' case to commit the line
			} else {
				t.current.Reset()
			}
		case '\n':
			t.lines = append(t.lines, t.current.String())
			t.current.Reset()
		case '\b':
			s := t.current.String()
			if s != "" {
				t.current.Reset()
				t.current.WriteString(s[:len(s)-1])
			}
		case '\a': // BEL, e.g. a stray terminal-bell byte: drop it
		default:
			if b >= 0x20 || b == '\t' {
				t.current.WriteByte(b)
			}
		}
	}
}

// String returns everything committed so far plus the in-progress line.
func (t *termBuffer) String() string {
	if t.current.Len() == 0 {
		return strings.Join(t.lines, "\n")
	}
	if len(t.lines) == 0 {
		return t.current.String()
	}
	return strings.Join(t.lines, "\n") + "\n" + t.current.String()
}

// keyMsgToBytes converts a keypress back into the raw bytes a real
// terminal would have sent, for forwarding to a child process attached to
// our pty. This only needs to cover what someone would plausibly type at
// an interactive apt/dpkg/sudo prompt: text, enter, backspace, ctrl+c, tab
// and arrows (some debconf dialogs are menu-driven).
func keyMsgToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyEnter:
		return []byte("\r")
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyTab:
		return []byte("\t")
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	default:
		return nil
	}
}

// runningProcess owns a privileged command attached to a pty: sudo and
// dpkg/debconf still see a real interactive terminal (same as before, when
// they had the whole screen), so password prompts and any unexpected
// interactive dialog keep working exactly as they did — only the display
// changes, into a bordered live-output box instead of taking over the
// whole terminal.
type runningProcess struct {
	cmd     *exec.Cmd
	ptmx    *os.File
	buf     termBuffer
	argv    []string
	backend string
	exited  bool
	exitErr error
}

func startPTYCmd(backend string, argv []string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command(argv[0], argv[1:]...)
		ptmx, err := pty.Start(c)
		if err != nil {
			return ptyStartedMsg{backend: backend, err: err}
		}
		return ptyStartedMsg{backend: backend, proc: &runningProcess{cmd: c, ptmx: ptmx, argv: argv, backend: backend}}
	}
}

type ptyStartedMsg struct {
	backend string
	proc    *runningProcess
	err     error
}

func (m ptyStartedMsg) Backend() string { return m.backend }

type ptyOutputMsg struct {
	backend string
	proc    *runningProcess
	data    []byte
	err     error
}

func (m ptyOutputMsg) Backend() string { return m.backend }

// readPTYCmd blocks for one Read() on the pty master. The caller re-issues
// it after each message to keep the pump going until the read errors out
// (io.EOF once the child closes its end, typically on exit).
func readPTYCmd(rp *runningProcess) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := rp.ptmx.Read(buf)
		var data []byte
		if n > 0 {
			data = append([]byte(nil), buf[:n]...)
		}
		return ptyOutputMsg{backend: rp.backend, proc: rp, data: data, err: err}
	}
}

type ptyExitMsg struct {
	backend string
	proc    *runningProcess
	err     error
}

func (m ptyExitMsg) Backend() string { return m.backend }

func waitPTYCmd(rp *runningProcess) tea.Cmd {
	return func() tea.Msg {
		err := rp.cmd.Wait()
		return ptyExitMsg{backend: rp.backend, proc: rp, err: err}
	}
}

// resize informs the child's pty of the new terminal size, so apt's own
// terminal-width-aware output (progress bars, wrapped text) isn't sized
// for a stale width.
func (rp *runningProcess) resize(cols, rows int) {
	if rp == nil || rp.ptmx == nil || cols <= 0 || rows <= 0 {
		return
	}
	_ = pty.Setsize(rp.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (rp *runningProcess) close() {
	if rp != nil && rp.ptmx != nil {
		_ = rp.ptmx.Close()
	}
}
