package ui

import (
	"strings"
	"time"
)

// logEntry is one completed privileged action, for the in-app log screen.
type logEntry struct {
	when    time.Time
	backend string
	summary string // human-readable, derived from the command's own argv
	ok      bool
	detail  string // error text, only set when !ok
}

// maxLogEntries caps sessionLog so a long-running session doesn't grow this
// without bound; the oldest entries are dropped first, since anything a
// user still cares about was almost certainly a recent action.
const maxLogEntries = 200

// sessionLog is every privileged action run since pkgtui started, appended
// to from Panel.dismissRunning — the single point that already knows both
// what ran and whether it succeeded. Deliberately a package-level var
// rather than a field threaded through App/Panel: the log screen shows
// both backends' history together regardless of which panel is active,
// same reasoning as the theme/keybinding globals elsewhere in this
// package.
var sessionLog []logEntry

// summarizeArgv turns a command's argv into a short human-readable label,
// stripping the "sudo" prefix MaybeSudo adds (redundant noise here — every
// entry in this log was already a privileged action by definition) and any
// leading "-y"/"--yes"-style flags aren't worth showing either.
func summarizeArgv(argv []string) string {
	rest := argv
	if len(rest) > 0 && rest[0] == "sudo" {
		rest = rest[1:]
	}
	return strings.Join(rest, " ")
}

// logAction records one completed action. exitErr is nil on success.
func logAction(backend string, argv []string, exitErr error) {
	entry := logEntry{
		when:    time.Now(),
		backend: backend,
		summary: summarizeArgv(argv),
		ok:      exitErr == nil,
	}
	if exitErr != nil {
		entry.detail = exitErr.Error()
	}
	sessionLog = append(sessionLog, entry)
	if len(sessionLog) > maxLogEntries {
		sessionLog = sessionLog[len(sessionLog)-maxLogEntries:]
	}
}
