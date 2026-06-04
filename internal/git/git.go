// Package git is a thin wrapper around shelling out to the git binary. becket
// is mostly orchestration over git, so this keeps invocations in one place.
// Because we exec the same git the bash script does, git's own output (worktree
// add messages, rebase progress, log lines) is byte-identical for free.
package git

import (
	"os"
	"os/exec"
	"strings"
)

// Output runs `git -C dir args...` and returns trimmed stdout. Stderr is
// discarded; callers that care about failure use the returned error.
func Output(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", prepend(dir, args)...).Output()
	return strings.TrimSpace(string(out)), err
}

// Run runs `git -C dir args...` with stdout/stderr connected to the process's
// own, so git's output streams through exactly as it does under bash.
func Run(dir string, args ...string) error {
	cmd := exec.Command("git", prepend(dir, args)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Quiet runs git discarding all output; used for predicate-style calls.
func Quiet(dir string, args ...string) error {
	return exec.Command("git", prepend(dir, args)...).Run()
}

// RunStdout streams stdout but discards stderr, matching bash `git … 2>/dev/null`
// where the command's stdout (e.g. "Deleted branch …") should still appear.
func RunStdout(dir string, args ...string) error {
	cmd := exec.Command("git", prepend(dir, args)...)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// Verify reports whether a ref resolves in the repo at dir (e.g. "origin/main").
func Verify(dir, ref string) bool {
	return Quiet(dir, "rev-parse", "--verify", ref) == nil
}

// CurrentBranch returns the checked-out branch name (empty if detached).
func CurrentBranch(dir string) string {
	out, _ := Output(dir, "branch", "--show-current")
	return out
}

// IsClean reports whether the working tree and index have no changes, matching
// the bash `diff --quiet && diff --cached --quiet` check.
func IsClean(dir string) bool {
	return Quiet(dir, "diff", "--quiet") == nil && Quiet(dir, "diff", "--cached", "--quiet") == nil
}

// CountRange returns the commit count for a `<range>` (e.g. "main..branch"),
// or "?" if it can't be computed, matching `rev-list --count`.
func CountRange(dir, rng string) string {
	out, err := Output(dir, "rev-list", "--count", rng)
	if err != nil {
		return "?"
	}
	return out
}

// LastCommitISO returns the committer date of ref in strict ISO-8601 (%cI), or
// "" if unavailable.
func LastCommitISO(dir, ref string) string {
	out, err := Output(dir, "log", "-1", "--format=%cI", ref)
	if err != nil {
		return ""
	}
	return out
}

func prepend(dir string, args []string) []string {
	return append([]string{"-C", dir}, args...)
}
