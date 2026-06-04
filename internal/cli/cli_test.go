package cli

import "testing"

// These tests cover only the pure helpers in this package — functions that take
// plain inputs and return values without touching loadPlatform, render.Die, the
// filesystem, git, or cobra. Each is table-driven and uses the standard testing
// package.

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"already a slug", "fix-bug-123", "fix-bug-123"},
		{"uppercase lowered", "FixBug", "fixbug"},
		{"spaces become hyphens", "fix the bug", "fix-the-bug"},
		{"mixed case with spaces", "Fix The Bug", "fix-the-bug"},
		{"strip punctuation", "fix: the bug!", "fix-the-bug"},
		{"strip everything but allowed", "Hello, World_2024", "hello-world2024"},
		{"digits kept", "v1.2.3", "v123"},
		{"underscores stripped", "snake_case_name", "snakecasename"},
		{"slashes stripped", "feat/new-thing", "featnew-thing"},
		{"leading/trailing spaces become hyphens", " hi ", "-hi-"},
		{"tabs are not spaces and get stripped", "a\tb", "ab"},
		{"non-ascii runes stripped", "café-déjà", "caf-dj"},
		{"only punctuation yields empty", "!@#$%^&*()", ""},
		{"emoji stripped", "release 🚀 now", "release--now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slugify(tt.in); got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"shorter than n unchanged", "hello", 10, "hello"},
		{"equal to n unchanged", "hello", 5, "hello"},
		{"one over n truncated", "hello!", 5, "he..."},
		{"long string truncated", "abcdefghij", 6, "abc..."},
		{"empty string unchanged", "", 5, ""},
		{"truncate to exactly n length", "abcdefghij", 7, "abcd..."},
		{"n=3 over length yields just ellipsis", "abcd", 3, "..."},
	}
	// NOTE: truncate has no guard for n < 3 on an over-length string: it would
	// evaluate s[:n-3] with a negative index and panic. The CLI only ever calls
	// it with sensible column widths (n >= 3), so those pathological inputs are
	// deliberately not exercised here.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.in, tt.n); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
		})
	}
}

func TestParseCount(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantVal int
	}{
		{"question mark is nil", "?", true, 0},
		{"zero", "0", false, 0},
		{"positive number", "42", false, 42},
		{"negative number", "-7", false, -7},
		{"junk is nil", "abc", true, 0},
		{"empty is nil", "", true, 0},
		{"trailing junk is nil", "12x", true, 0},
		{"leading space is nil", " 12", true, 0},
		{"float is nil", "1.5", true, 0},
		{"plus-prefixed number", "+3", false, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCount(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Errorf("parseCount(%q) = %v (%d), want nil", tt.in, got, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseCount(%q) = nil, want *int(%d)", tt.in, tt.wantVal)
			}
			if *got != tt.wantVal {
				t.Errorf("parseCount(%q) = %d, want %d", tt.in, *got, tt.wantVal)
			}
		})
	}
}

func TestBuildEnvPrefix(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"nil map", nil, ""},
		{"empty map", map[string]string{}, ""},
		{"single entry", map[string]string{"FOO": "bar"}, "export FOO=bar && "},
		{
			"keys sorted",
			map[string]string{"ZED": "1", "ALPHA": "2", "MID": "3"},
			"export ALPHA=2 && export MID=3 && export ZED=1 && ",
		},
		{
			"two entries sorted",
			map[string]string{"B": "2", "A": "1"},
			"export A=1 && export B=2 && ",
		},
		{
			"empty value preserved",
			map[string]string{"EMPTY": ""},
			"export EMPTY= && ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildEnvPrefix(tt.env); got != tt.want {
				t.Errorf("buildEnvPrefix(%v) = %q, want %q", tt.env, got, tt.want)
			}
		})
	}
}

func TestBuildEnvPrefixDeterministic(t *testing.T) {
	// Map iteration order is randomized in Go, so verify the sorted output is
	// stable across many runs of the same input.
	env := map[string]string{"D": "4", "A": "1", "C": "3", "B": "2"}
	want := "export A=1 && export B=2 && export C=3 && export D=4 && "
	for i := 0; i < 100; i++ {
		if got := buildEnvPrefix(env); got != want {
			t.Fatalf("iteration %d: buildEnvPrefix = %q, want %q", i, got, want)
		}
	}
}

func TestNext(t *testing.T) {
	tests := []struct {
		name string
		args []string
		i    int
		want string
	}{
		{"in range middle", []string{"a", "b", "c"}, 0, "b"},
		{"in range last pair", []string{"a", "b", "c"}, 1, "c"},
		{"out of range at end", []string{"a", "b", "c"}, 2, ""},
		{"out of range past end", []string{"a", "b", "c"}, 5, ""},
		{"nil args", nil, 0, ""},
		{"empty args", []string{}, 0, ""},
		{"single element no next", []string{"only"}, 0, ""},
		{"negative index returns args[i+1]", []string{"x", "y"}, -1, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := next(tt.args, tt.i); got != tt.want {
				t.Errorf("next(%v, %d) = %q, want %q", tt.args, tt.i, got, tt.want)
			}
		})
	}
}

func TestArg0(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"first of many", []string{"first", "second"}, "first"},
		{"single", []string{"only"}, "only"},
		{"empty slice", []string{}, ""},
		{"nil slice", nil, ""},
		{"empty first element preserved", []string{""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := arg0(tt.args); got != tt.want {
				t.Errorf("arg0(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
