package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mykhol/becket/internal/config"
)

func TestMatchesAnyGlob(t *testing.T) {
	patterns := []string{"spike-*", "*review*"}
	tests := []struct {
		id   string
		want bool
	}{
		{"spike-foo", true},
		{"spike-", true},
		{"mul-42-review", true},
		{"review", true},
		{"mul-42", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := matchesAnyGlob(tt.id, patterns); got != tt.want {
			t.Errorf("matchesAnyGlob(%q, %v) = %v, want %v", tt.id, patterns, got, tt.want)
		}
	}
}

func TestSafeToPrune(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wt, "tracked_dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wt
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	git("init", "-q")
	git("config", "user.email", "test@becket.invalid")
	git("config", "user.name", "Becket Tester")
	if err := os.WriteFile(filepath.Join(wt, "tracked_dir", "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "tracked_dir/keep.txt")
	git("commit", "-q", "-m", "seed")

	if !safeToPrune(wt, filepath.Join(wt, "node_modules")) {
		t.Error("safeToPrune(untracked dir inside worktree) = false, want true")
	}
	if safeToPrune(wt, filepath.Join(wt, "tracked_dir")) {
		t.Error("safeToPrune(dir holding a tracked file) = true, want false")
	}
	if safeToPrune(wt, filepath.Dir(wt)) {
		t.Error("safeToPrune(a path outside the worktree) = true, want false")
	}
	if safeToPrune(wt, wt) {
		t.Error("safeToPrune(the worktree root itself) = true, want false")
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("dedupeStrings = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedupeStrings = %v, want %v", got, want)
		}
	}
}

func TestUnsavedWorkspaceFile(t *testing.T) {
	platform := t.TempDir()
	ws := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(platform, ".tasks", "plan.md"), "platform")
	write(filepath.Join(ws, ".tasks", "plan.md"), "platform")
	write(filepath.Join(ws, "AGENTS.md"), "generated")
	write(filepath.Join(ws, "repo", "README.md"), "code")
	if err := os.MkdirAll(filepath.Join(ws, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := &config.Platform{Dir: platform, Settings: config.Settings{Files: []string{".tasks"}}}
	repos := []gcRepoInfo{{name: "repo"}}

	if got := unsavedWorkspaceFile(p, ws, repos); got != "" {
		t.Fatalf("pristine workspace reported %q", got)
	}

	write(filepath.Join(ws, "docs", "notes.md"), "notes")
	if got := unsavedWorkspaceFile(p, ws, repos); got != "docs" {
		t.Fatalf("docs with a file: got %q, want docs", got)
	}
	_ = os.RemoveAll(filepath.Join(ws, "docs"))

	write(filepath.Join(ws, ".reports", "plan.md"), "report")
	if got := unsavedWorkspaceFile(p, ws, repos); got != ".reports" {
		t.Fatalf("unknown root entry: got %q, want .reports", got)
	}
	_ = os.RemoveAll(filepath.Join(ws, ".reports"))

	future := time.Now().Add(time.Hour)
	write(filepath.Join(ws, ".tasks", "plan.md"), "edited in workspace")
	_ = os.Chtimes(filepath.Join(ws, ".tasks", "plan.md"), future, future)
	if got := unsavedWorkspaceFile(p, ws, repos); got != ".tasks" {
		t.Fatalf("seeded file edited in workspace: got %q, want .tasks", got)
	}

	write(filepath.Join(platform, ".tasks", "plan.md"), "platform moved on")
	_ = os.Chtimes(filepath.Join(platform, ".tasks", "plan.md"), future.Add(time.Hour), future.Add(time.Hour))
	if got := unsavedWorkspaceFile(p, ws, repos); got != "" {
		t.Fatalf("platform-side change reported as workspace edit: %q", got)
	}
}
