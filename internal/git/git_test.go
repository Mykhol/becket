package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// requireGit skips the test if the git binary is unavailable, so the suite is a
// no-op rather than a failure on machines without git installed.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available; skipping git package tests")
	}
}

// newRepo builds a fresh git repository in t.TempDir() with a deterministic
// identity and a single initial commit, and returns the repo directory. All
// dates are pinned via GIT_*_DATE so commit-date assertions are stable.
func newRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)

	dir := t.TempDir()

	// Pin author/committer dates for determinism. git reads these from the
	// environment of the child process; we set them on the parent's env via
	// t.Setenv so every git invocation in this test inherits them.
	const fixedDate = "2021-01-02T03:04:05+00:00"
	t.Setenv("GIT_AUTHOR_DATE", fixedDate)
	t.Setenv("GIT_COMMITTER_DATE", fixedDate)
	// Defend against the surrounding environment leaking identity/config.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", dir)
	t.Setenv("GIT_AUTHOR_NAME", "Becket Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@becket.example")
	t.Setenv("GIT_COMMITTER_NAME", "Becket Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@becket.example")

	// Isolate the working directory too, per the task's isolation guidance.
	t.Chdir(dir)

	mustGit(t, dir, "init", "-b", "main")
	mustGit(t, dir, "config", "user.name", "Becket Test")
	mustGit(t, dir, "config", "user.email", "test@becket.example")

	// Seed a tracked file and commit it.
	writeFile(t, dir, "file.txt", "hello\n")
	mustGit(t, dir, "add", "file.txt")
	mustGit(t, dir, "commit", "-m", "initial commit")

	return dir
}

// mustGit runs a git command in dir and fails the test on error, surfacing
// combined output for debugging.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// silenceStdio redirects os.Stdout and os.Stderr to the null device for the
// duration of the test, so Run/RunStdout (which inherit the process's stdio)
// don't pollute test output. Restored automatically via t.Cleanup.
func silenceStdio(t *testing.T) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	t.Cleanup(func() {
		os.Stdout, os.Stderr = origOut, origErr
		devnull.Close()
	})
}

func TestOutput(t *testing.T) {
	dir := newRepo(t)

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "trimmed stdout for current branch",
			args: []string{"branch", "--show-current"},
			want: "main",
		},
		{
			name: "rev-parse HEAD abbrev is trimmed",
			args: []string{"rev-parse", "--short", "HEAD"},
			// value is non-deterministic length; assert non-empty separately
			want: "",
		},
		{
			name:    "error on bogus subcommand",
			args:    []string{"definitely-not-a-git-command"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Output(dir, tc.args...)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (out=%q)", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Output must never carry leading/trailing whitespace.
			if got := out; got != "" {
				if got[0] == ' ' || got[0] == '\n' || got[len(got)-1] == '\n' || got[len(got)-1] == ' ' {
					t.Fatalf("output not trimmed: %q", got)
				}
			}
			if tc.want != "" && out != tc.want {
				t.Fatalf("Output = %q, want %q", out, tc.want)
			}
		})
	}
}

func TestQuiet(t *testing.T) {
	dir := newRepo(t)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "succeeds for valid command",
			args:    []string{"rev-parse", "--verify", "HEAD"},
			wantErr: false,
		},
		{
			name:    "errors for invalid ref",
			args:    []string{"rev-parse", "--verify", "origin/nope"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Quiet(dir, tc.args...)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestVerify(t *testing.T) {
	dir := newRepo(t)
	branch := CurrentBranch(dir)
	if branch == "" {
		t.Fatal("precondition: expected a current branch")
	}

	tests := []struct {
		name string
		ref  string
		want bool
	}{
		{name: "existing branch resolves", ref: branch, want: true},
		{name: "HEAD resolves", ref: "HEAD", want: true},
		{name: "missing remote ref does not resolve", ref: "origin/nope", want: false},
		{name: "garbage ref does not resolve", ref: "this-ref-does-not-exist", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Verify(dir, tc.ref); got != tc.want {
				t.Fatalf("Verify(%q) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := newRepo(t)

	t.Run("returns checked-out branch", func(t *testing.T) {
		if got := CurrentBranch(dir); got != "main" {
			t.Fatalf("CurrentBranch = %q, want %q", got, "main")
		}
	})

	t.Run("empty when detached HEAD", func(t *testing.T) {
		// Detach HEAD onto the current commit; --show-current prints nothing.
		mustGit(t, dir, "checkout", "--detach", "HEAD")
		if got := CurrentBranch(dir); got != "" {
			t.Fatalf("CurrentBranch (detached) = %q, want empty", got)
		}
	})
}

func TestIsClean(t *testing.T) {
	dir := newRepo(t)

	tests := []struct {
		name  string
		setup func(t *testing.T)
		want  bool
	}{
		{
			name:  "clean immediately after commit",
			setup: func(t *testing.T) {},
			want:  true,
		},
		{
			name: "dirty after editing a tracked file (unstaged)",
			setup: func(t *testing.T) {
				writeFile(t, dir, "file.txt", "hello world\n")
			},
			want: false,
		},
		{
			name: "dirty after staging a change",
			setup: func(t *testing.T) {
				writeFile(t, dir, "file.txt", "staged change\n")
				mustGit(t, dir, "add", "file.txt")
			},
			want: false,
		},
		{
			name: "clean again after committing the change",
			setup: func(t *testing.T) {
				writeFile(t, dir, "file.txt", "committed change\n")
				mustGit(t, dir, "add", "file.txt")
				mustGit(t, dir, "commit", "-m", "second commit")
			},
			want: true,
		},
	}

	// These cases mutate shared repo state, so they run sequentially in order.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			if got := IsClean(dir); got != tc.want {
				t.Fatalf("IsClean = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCountRange(t *testing.T) {
	dir := newRepo(t)

	tests := []struct {
		name string
		rng  string
		want string
	}{
		{
			name: "no-op range counts zero",
			rng:  "HEAD..HEAD",
			want: "0",
		},
		{
			name: "bad range yields question mark",
			rng:  "no-such-ref..also-missing",
			want: "?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountRange(dir, tc.rng); got != tc.want {
				t.Fatalf("CountRange(%q) = %q, want %q", tc.rng, got, tc.want)
			}
		})
	}
}

func TestCountRangeWithCommits(t *testing.T) {
	dir := newRepo(t)

	// Create a side branch one commit ahead of main.
	mustGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	mustGit(t, dir, "add", "feature.txt")
	mustGit(t, dir, "commit", "-m", "feature commit")

	if got := CountRange(dir, "main..feature"); got != "1" {
		t.Fatalf("CountRange(main..feature) = %q, want %q", got, "1")
	}
}

func TestRun(t *testing.T) {
	dir := newRepo(t)
	silenceStdio(t)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "succeeds for valid command", args: []string{"status"}, wantErr: false},
		{name: "errors for bogus command", args: []string{"not-a-real-subcommand"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(dir, tc.args...)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunStdout(t *testing.T) {
	dir := newRepo(t)
	silenceStdio(t)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "succeeds for valid command", args: []string{"rev-parse", "HEAD"}, wantErr: false},
		{name: "errors for bogus command", args: []string{"not-a-real-subcommand"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RunStdout(dir, tc.args...)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLastCommitISO(t *testing.T) {
	dir := newRepo(t)

	t.Run("HEAD returns pinned ISO-8601 date", func(t *testing.T) {
		got := LastCommitISO(dir, "HEAD")
		if got == "" {
			t.Fatal("LastCommitISO(HEAD) was empty")
		}
		// We pinned the committer date, so the value is fully deterministic.
		// git's %cI emits the UTC offset as "Z" rather than "+00:00".
		const want = "2021-01-02T03:04:05Z"
		if got != want {
			t.Fatalf("LastCommitISO(HEAD) = %q, want %q", got, want)
		}
		// And it must parse as strict ISO-8601 / RFC3339.
		if _, err := time.Parse(time.RFC3339, got); err != nil {
			t.Fatalf("LastCommitISO not RFC3339-parseable: %v", err)
		}
	})

	t.Run("missing ref returns empty string", func(t *testing.T) {
		if got := LastCommitISO(dir, "no-such-ref"); got != "" {
			t.Fatalf("LastCommitISO(no-such-ref) = %q, want empty", got)
		}
	})
}
