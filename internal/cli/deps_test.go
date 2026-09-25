package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/workspace"
)

func TestHasGlobMeta(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"node_modules", false},
		{".venv", false},
		{"apps/web", false},
		{"apps/*/node_modules", true},
		{"build-?", true},
		{"[abc]", true},
	}
	for _, tt := range tests {
		if got := hasGlobMeta(tt.in); got != tt.want {
			t.Errorf("hasGlobMeta(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestDepsKey(t *testing.T) {
	wt := t.TempDir()
	dc := &config.DepsConfig{
		Run:       []string{"pnpm install"},
		Lockfiles: []string{"pnpm-lock.yaml"},
	}

	// Missing lockfile hashes deterministically to a stable "missing" key.
	key1 := depsKey(wt, dc)
	key2 := depsKey(wt, dc)
	if key1 != key2 {
		t.Errorf("depsKey is not deterministic: %q != %q", key1, key2)
	}

	// Writing the lockfile changes the key.
	if err := os.WriteFile(filepath.Join(wt, "pnpm-lock.yaml"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	key3 := depsKey(wt, dc)
	if key3 == key1 {
		t.Error("depsKey unchanged after lockfile appeared")
	}

	// Changing the lockfile's contents changes the key again.
	if err := os.WriteFile(filepath.Join(wt, "pnpm-lock.yaml"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	key4 := depsKey(wt, dc)
	if key4 == key3 {
		t.Error("depsKey unchanged after lockfile contents changed")
	}

	// Changing a run command changes the key even with the lockfile untouched.
	dc2 := &config.DepsConfig{Run: []string{"pnpm install --frozen-lockfile"}, Lockfiles: dc.Lockfiles}
	key5 := depsKey(wt, dc2)
	if key5 == key4 {
		t.Error("depsKey unchanged after a run command changed")
	}
}

func TestDepsUpToDate(t *testing.T) {
	ws := t.TempDir()
	wt := t.TempDir()
	const repo = "service"
	dc := &config.DepsConfig{
		Run:  []string{"mkdir -p node_modules"},
		Dirs: []string{"node_modules", "apps/*/node_modules"},
	}

	if depsUpToDate(ws, repo, wt, dc) {
		t.Error("depsUpToDate = true with no stamp written, want false")
	}

	if err := installRepoDeps(ws, repo, dc, wt, "", false); err != nil {
		t.Fatalf("installRepoDeps: %v", err)
	}
	if !depsUpToDate(ws, repo, wt, dc) {
		t.Error("depsUpToDate = false right after a successful install, want true")
	}

	// A missing non-glob Dirs entry invalidates the stamp even though the key
	// still matches (e.g. someone deleted node_modules by hand).
	if err := os.RemoveAll(filepath.Join(wt, "node_modules")); err != nil {
		t.Fatal(err)
	}
	if depsUpToDate(ws, repo, wt, dc) {
		t.Error("depsUpToDate = true after a configured Dirs entry was removed, want false")
	}
}

func TestReposForDeps(t *testing.T) {
	ws := t.TempDir()
	for _, repo := range []string{"api", "web"} {
		if err := os.MkdirAll(filepath.Join(ws, repo), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	m := &workspace.Manifest{
		Order: []string{"api", "web"},
		Repos: map[string]workspace.RepoEntry{"api": {}, "web": {}},
	}

	if got := reposForDeps(ws, m, "api"); len(got) != 1 || got[0] != "api" {
		t.Errorf("reposForDeps with --repo = %v, want [api]", got)
	}

	if got := reposForDeps(ws, m, ""); len(got) != 2 {
		t.Errorf("reposForDeps with no cwd match and no flag = %v, want both repos", got)
	}
}
