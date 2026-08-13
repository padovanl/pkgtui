package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTermBufferPlainLines(t *testing.T) {
	var b termBuffer
	b.Write([]byte("Reading package lists...\nBuilding dependency tree...\n"))
	want := "Reading package lists...\nBuilding dependency tree..."
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTermBufferCRLFIsJustANewline(t *testing.T) {
	// A pty in cooked mode rewrites outgoing '\n' to "\r\n" (ONLCR); that
	// trailing \r must NOT be treated as a progress-bar reset, or every
	// line of real output silently comes out blank.
	var b termBuffer
	b.Write([]byte("Reading package lists...\r\nBuilding dependency tree...\r\n"))
	want := "Reading package lists...\nBuilding dependency tree..."
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTermBufferCRLFSplitAcrossWrites(t *testing.T) {
	// Same as above, but the \r and \n land in separate Read() chunks.
	var b termBuffer
	b.Write([]byte("Reading package lists...\r"))
	b.Write([]byte("\nBuilding dependency tree...\r"))
	b.Write([]byte("\n"))
	want := "Reading package lists...\nBuilding dependency tree..."
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTermBufferStandaloneCRAcrossWrites(t *testing.T) {
	// A progress-bar \r (not part of a CRLF pair) split across chunks must
	// still behave as a reset, not get mistaken for a pending CRLF.
	var b termBuffer
	b.Write([]byte("50%\r"))
	b.Write([]byte("100% done\n"))
	want := "100% done"
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTermBufferCarriageReturnCollapsesProgress(t *testing.T) {
	var b termBuffer
	// apt-style progress: repeated \r-terminated updates on one line,
	// finished with a real \n once the step completes.
	b.Write([]byte("Reading package lists... 0%\r"))
	b.Write([]byte("Reading package lists... 45%\r"))
	b.Write([]byte("Reading package lists... Done\n"))
	want := "Reading package lists... Done"
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTermBufferBackspace(t *testing.T) {
	var b termBuffer
	b.Write([]byte("abc\bd")) // "abc", backspace over 'c', then 'd' -> "abd"
	if got := b.String(); got != "abd" {
		t.Errorf("String() = %q, want %q", got, "abd")
	}
}

func TestTermBufferStripsANSIEscapes(t *testing.T) {
	var b termBuffer
	// SGR color codes and a cursor-movement sequence mixed into real text,
	// as apt/dpkg output commonly does.
	b.Write([]byte("\x1b[32mOK\x1b[0m \x1b[1;1Hstatus: \x1b[33minstalling\x1b[0m\n"))
	want := "OK status: installing"
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTermBufferSplitAcrossWrites(t *testing.T) {
	var b termBuffer
	// A read loop delivers data in arbitrary chunks; the buffer must not
	// assume a whole line arrives in one Write call.
	b.Write([]byte("part"))
	b.Write([]byte("ial line"))
	b.Write([]byte("\ndone\n"))
	want := "partial line\ndone"
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestKeyMsgToBytes(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
		want string
	}{
		{"runes", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hunter2")}, "hunter2"},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, "\r"},
		{"space", tea.KeyMsg{Type: tea.KeySpace}, " "},
		{"backspace", tea.KeyMsg{Type: tea.KeyBackspace}, "\x7f"},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}, "\x03"},
		{"up", tea.KeyMsg{Type: tea.KeyUp}, "\x1b[A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(keyMsgToBytes(c.msg))
			if got != c.want {
				t.Errorf("keyMsgToBytes(%v) = %q, want %q", c.msg.Type, got, c.want)
			}
		})
	}
}
