package render

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestPadRight(t *testing.T) {
	// "─" is the box-drawing char U+2500, which is 3 bytes but a single rune.
	const box = "─" // ─

	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{
			name: "empty string padded to width",
			s:    "",
			n:    3,
			want: "   ",
		},
		{
			name: "ascii padded with spaces",
			s:    "ab",
			n:    5,
			want: "ab   ",
		},
		{
			name: "ascii exact width no padding",
			s:    "abc",
			n:    3,
			want: "abc",
		},
		{
			name: "ascii longer than width not truncated",
			s:    "abcdef",
			n:    3,
			want: "abcdef",
		},
		{
			name: "zero width returns string unchanged",
			s:    "abc",
			n:    0,
			want: "abc",
		},
		{
			name: "negative width returns string unchanged",
			s:    "abc",
			n:    -5,
			want: "abc",
		},
		{
			name: "multibyte box char counts as one rune, padded by runes",
			// One "─" rune (3 bytes). Padding to 4 runes => 3 trailing spaces.
			s:    box,
			n:    4,
			want: box + "   ",
		},
		{
			name: "multiple box chars counted as runes",
			// Three "─" runes (9 bytes). Pad to 5 runes => 2 trailing spaces.
			s:    box + box + box,
			n:    5,
			want: box + box + box + "  ",
		},
		{
			name: "multibyte exact rune width no padding",
			s:    box + box,
			n:    2,
			want: box + box,
		},
		{
			name: "multibyte longer than width not truncated",
			// Four runes, requesting width 2 => unchanged (no truncation).
			s:    box + box + box + box,
			n:    2,
			want: box + box + box + box,
		},
		{
			name: "mixed ascii and multibyte runes",
			// "a─b" is 3 runes; pad to 5 runes => 2 trailing spaces.
			s:    "a" + box + "b",
			n:    5,
			want: "a" + box + "b" + "  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.s, tt.n)
			if got != tt.want {
				t.Fatalf("PadRight(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}

			// The result must always count at least max(n, runeLen(s)) runes,
			// and must never be shorter than the input (no truncation).
			gotRunes := utf8.RuneCountInString(got)
			srcRunes := utf8.RuneCountInString(tt.s)
			wantRunes := srcRunes
			if tt.n > wantRunes {
				wantRunes = tt.n
			}
			if gotRunes != wantRunes {
				t.Errorf("PadRight(%q, %d) rune count = %d, want %d", tt.s, tt.n, gotRunes, wantRunes)
			}

			// PadRight only ever appends; the original string must be a prefix.
			if !strings.HasPrefix(got, tt.s) {
				t.Errorf("PadRight(%q, %d) = %q does not start with input", tt.s, tt.n, got)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	// Build timestamps relative to now so the test is independent of wall clock.
	// Offsets are chosen comfortably inside each band (away from boundaries) so
	// the int-second truncation in RelativeTime cannot race with test timing.
	ago := func(d time.Duration) string {
		return time.Now().Add(-d).Format(time.RFC3339)
	}

	tests := []struct {
		name string
		iso  string
		want string
	}{
		{
			name: "empty input returns empty",
			iso:  "",
			want: "",
		},
		{
			name: "unparseable input returns empty",
			iso:  "not-a-timestamp",
			want: "",
		},
		{
			name: "non-rfc3339 date-only string returns empty",
			iso:  "2026-06-04",
			want: "",
		},
		{
			name: "zero seconds is just now",
			iso:  ago(0),
			want: "just now",
		},
		{
			name: "30s ago is just now (under 60s)",
			iso:  ago(30 * time.Second),
			want: "just now",
		},
		{
			name: "90s ago is 1 min ago",
			iso:  ago(90 * time.Second),
			want: "1 min ago",
		},
		{
			name: "30 min ago",
			iso:  ago(30 * time.Minute),
			want: "30 min ago",
		},
		{
			name: "59 min ago (just under 1h)",
			iso:  ago(59 * time.Minute),
			want: "59 min ago",
		},
		{
			name: "90 min ago is 1h ago",
			iso:  ago(90 * time.Minute),
			want: "1h ago",
		},
		{
			name: "5h ago",
			iso:  ago(5 * time.Hour),
			want: "5h ago",
		},
		{
			name: "23h ago (just under 1 day)",
			iso:  ago(23 * time.Hour),
			want: "23h ago",
		},
		{
			name: "36h ago is 1d ago",
			iso:  ago(36 * time.Hour),
			want: "1d ago",
		},
		{
			name: "3d ago",
			iso:  ago(3 * 24 * time.Hour),
			want: "3d ago",
		},
		{
			name: "6d ago (just under 1 week)",
			iso:  ago(6 * 24 * time.Hour),
			want: "6d ago",
		},
		{
			name: "10d ago is 1w ago",
			iso:  ago(10 * 24 * time.Hour),
			want: "1w ago",
		},
		{
			name: "3w ago",
			iso:  ago(3 * 7 * 24 * time.Hour),
			want: "3w ago",
		},
		{
			name: "52w ago",
			iso:  ago(52 * 7 * 24 * time.Hour),
			want: "52w ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelativeTime(tt.iso)
			if got != tt.want {
				t.Fatalf("RelativeTime(%q) = %q, want %q", tt.iso, got, tt.want)
			}
		})
	}
}

// TestRelativeTimeRFC3339WithZone confirms that a timezone-offset RFC3339
// timestamp parses and is normalized to UTC for the elapsed-time math, so the
// band is computed from the absolute instant (not the local wall-clock string).
func TestRelativeTimeRFC3339WithZone(t *testing.T) {
	// Same instant, two equivalent RFC3339 spellings.
	instant := time.Now().Add(-2 * time.Hour)
	utc := instant.UTC().Format(time.RFC3339)
	offset := instant.In(time.FixedZone("plus5", 5*60*60)).Format(time.RFC3339)

	if got := RelativeTime(utc); got != "2h ago" {
		t.Errorf("RelativeTime(%q) = %q, want %q", utc, got, "2h ago")
	}
	if got := RelativeTime(offset); got != "2h ago" {
		t.Errorf("RelativeTime(%q) = %q, want %q", offset, got, "2h ago")
	}
}
