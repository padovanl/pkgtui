//go:build manualptycheck

package ui

// Interactive harness to exercise the real pty plumbing
// (startPTYCmd/readPTYCmd/waitPTYCmd/keyMsgToBytes) against a script that
// simulates a no-echo password prompt and a y/n prompt, without needing
// real sudo credentials. Not part of the normal test suite (build-tagged
// out); run manually inside a real terminal/tmux pane:
//
//	go test -tags manualptycheck -run TestManualPTYInteractive -v ./internal/ui/ -args <script-path>

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type ptyCheckModel struct {
	proc *runningProcess
	done bool
	err  error
}

func (m ptyCheckModel) Init() tea.Cmd {
	return startPTYCmd("test", []string{os.Args[len(os.Args)-1]})
}

func (m ptyCheckModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ptyStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.done = true
			return m, tea.Quit
		}
		m.proc = msg.proc
		return m, tea.Batch(readPTYCmd(m.proc), waitPTYCmd(m.proc))
	case ptyOutputMsg:
		if len(msg.data) > 0 {
			m.proc.buf.Write(msg.data)
		}
		if msg.err != nil {
			return m, nil
		}
		return m, readPTYCmd(m.proc)
	case ptyExitMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC && m.proc == nil {
			return m, tea.Quit
		}
		if m.proc != nil {
			if b := keyMsgToBytes(msg); b != nil {
				_, _ = m.proc.ptmx.Write(b)
			}
		}
	}
	return m, nil
}

func (m ptyCheckModel) View() string {
	body := ""
	if m.proc != nil {
		body = m.proc.buf.String()
	}
	status := "running"
	if m.done {
		status = fmt.Sprintf("exited, err=%v", m.err)
	}
	return "=== pty check (" + status + ") ===\n" + body + "\n"
}

func TestManualPTYInteractive(t *testing.T) {
	if len(os.Args) == 0 || !strings.HasSuffix(os.Args[len(os.Args)-1], ".sh") {
		t.Skip("pass a script path via -args, e.g. -args /path/to/script.sh")
	}
	p := tea.NewProgram(ptyCheckModel{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		t.Fatalf("program error: %v", err)
	}
}
