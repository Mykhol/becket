// Package render centralizes becket's user-facing output: the info/warn/die
// helpers and the TTY-gated coloring, matching the bash script's conventions
// (info/warn → stdout with a "▸ " prefix; die → stderr "error: …", exit 1).
package render

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// ANSI codes, emptied when stdout is not a terminal — mirrors the bash
// `[[ -t 1 ]]` gate, so captured/piped output is plain (no NO_COLOR handling,
// same as the original).
var (
	green  string
	yellow string
	red    string
	reset  string
)

func init() {
	if isTTY(os.Stdout) {
		green = "\033[0;32m"
		yellow = "\033[0;33m"
		red = "\033[0;31m"
		reset = "\033[0m"
	}
}

func isTTY(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Info prints a "▸ " status line to stdout.
func Info(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "%s▸%s %s\n", green, reset, fmt.Sprintf(format, a...))
}

// Warn prints a "▸ " line to stdout (same glyph as Info; only the color
// differs on a TTY, matching the bash helpers).
func Warn(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "%s▸%s %s\n", yellow, reset, fmt.Sprintf(format, a...))
}

// Die prints "error: …" to stderr and exits non-zero.
func Die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%serror:%s %s\n", red, reset, fmt.Sprintf(format, a...))
	os.Exit(1)
}

// IsTTY reports whether stdout is a terminal (for the table renderers, which
// gate color the same way as the bash python helpers).
func IsTTY() bool { return isTTY(os.Stdout) }

// PadRight left-justifies s to n columns counting runes (not bytes), so the
// box-drawing characters in table separators align like Python's f-string width.
func PadRight(s string, n int) string {
	if pad := n - utf8.RuneCountInString(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// RelativeTime renders an ISO-8601 timestamp as becket's short relative form,
// matching the thresholds in the bash relative_time helper.
func RelativeTime(iso string) string {
	if iso == "" {
		return ""
	}
	ts, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	secs := int(time.Since(ts).Seconds())
	switch {
	case secs < 60:
		return "just now"
	case secs < 3600:
		return fmt.Sprintf("%d min ago", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh ago", secs/3600)
	case secs < 604800:
		return fmt.Sprintf("%dd ago", secs/86400)
	default:
		return fmt.Sprintf("%dw ago", secs/604800)
	}
}
