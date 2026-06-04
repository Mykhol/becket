package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// clearBecketConfig ensures BECKET_CONFIG is unset for a test so CWD-walking
// discovery is exercised. t.Setenv registers cleanup that restores the prior
// value (whether it was set or unset) at test end.
func clearBecketConfig(t *testing.T) {
	t.Helper()
	t.Setenv("BECKET_CONFIG", "")
	if err := os.Unsetenv("BECKET_CONFIG"); err != nil {
		t.Fatalf("unset BECKET_CONFIG: %v", err)
	}
}

// writeConfig creates <root>/.becket/settings.json containing body and returns
// the settings.json path.
func writeConfig(t *testing.T, root, body string) string {
	t.Helper()
	becketDir := filepath.Join(root, ".becket")
	if err := os.MkdirAll(becketDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", becketDir, err)
	}
	path := filepath.Join(becketDir, "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

const minimalConfig = `{
  "repos": {},
  "branchPrefix": "feat/"
}
`

func TestFind_HonorsBecketConfigEnv(t *testing.T) {
	// BECKET_CONFIG is returned verbatim and short-circuits the CWD walk: the
	// path need not even exist, and CWD is irrelevant.
	tmp := t.TempDir()
	want := filepath.Join(tmp, "anywhere", "custom-settings.json")
	t.Setenv("BECKET_CONFIG", want)

	// Chdir somewhere with no .becket to prove the env wins over discovery.
	other := t.TempDir()
	t.Chdir(other)

	got, err := Find()
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("Find() = %q, want %q", got, want)
	}
}

func TestFind_WalksUpFromCWD(t *testing.T) {
	clearBecketConfig(t)

	root := t.TempDir()
	cfgPath := writeConfig(t, root, minimalConfig)

	tests := []struct {
		name    string
		startAt string // directory (relative to root) to chdir into before Find
	}{
		{name: "platform root itself", startAt: "."},
		{name: "one level down", startAt: "sub"},
		{name: "several levels down", startAt: "sub/a/b/c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := filepath.Join(root, tt.startAt)
			if err := os.MkdirAll(start, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", start, err)
			}
			t.Chdir(start)

			got, err := Find()
			if err != nil {
				t.Fatalf("Find() error = %v, want nil", err)
			}
			// macOS temp dirs live under /private/var via a symlink; resolve
			// both sides so the comparison is robust to that.
			gotResolved := resolve(t, got)
			wantResolved := resolve(t, cfgPath)
			if gotResolved != wantResolved {
				t.Errorf("Find() = %q, want %q", gotResolved, wantResolved)
			}
		})
	}
}

func TestFind_NotFound(t *testing.T) {
	clearBecketConfig(t)

	// A fresh temp dir with no .becket anywhere up to the filesystem root.
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := Find()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Find() error = %v, want ErrNotFound", err)
	}
}

func TestFind_IgnoresDirectoryNamedSettingsJSON(t *testing.T) {
	// Find requires a regular file: a directory named settings.json must be
	// skipped (the !fi.IsDir() guard), so discovery falls through to ErrNotFound.
	clearBecketConfig(t)

	root := t.TempDir()
	bogus := filepath.Join(root, ".becket", "settings.json")
	if err := os.MkdirAll(bogus, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", bogus, err)
	}
	t.Chdir(root)

	_, err := Find()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Find() error = %v, want ErrNotFound (directory should not match)", err)
	}
}

func TestLoad_DerivesPathsAndSettings(t *testing.T) {
	clearBecketConfig(t)

	root := t.TempDir()
	body := `{
  "$schema": "https://example.com/becket.schema.json",
  "repos": {
    "api": { "path": "repos/api", "defaultBase": "main" },
    "web": { "path": "repos/web", "defaultBase": "develop" }
  },
  "branchPrefix": "feat/",
  "files": [".env"],
  "docker": "compose.yaml",
  "session": "becket"
}
`
	cfgPath := writeConfig(t, root, body)
	t.Chdir(root)

	p, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	wantDir := resolve(t, root)
	if got := resolve(t, p.ConfigFile); got != resolve(t, cfgPath) {
		t.Errorf("ConfigFile = %q, want %q", got, resolve(t, cfgPath))
	}
	if got := resolve(t, p.Dir); got != wantDir {
		t.Errorf("Dir = %q, want %q (parent of .becket)", got, wantDir)
	}
	// WorkspacesDir is derived purely from p.Dir, so assert that relationship
	// directly rather than resolving symlinks (the dir does not exist on disk).
	wantWorkspaces := filepath.Join(p.Dir, ".becket", "workspaces")
	if p.WorkspacesDir != wantWorkspaces {
		t.Errorf("WorkspacesDir = %q, want %q", p.WorkspacesDir, wantWorkspaces)
	}
	if !p.HasSchema {
		t.Errorf("HasSchema = false, want true ($schema present)")
	}
	if p.Settings.BranchPrefix != "feat/" {
		t.Errorf("BranchPrefix = %q, want %q", p.Settings.BranchPrefix, "feat/")
	}
	if p.Settings.Docker != "compose.yaml" {
		t.Errorf("Docker = %q, want %q", p.Settings.Docker, "compose.yaml")
	}
	if p.Settings.Session != "becket" {
		t.Errorf("Session = %q, want %q", p.Settings.Session, "becket")
	}
	if !reflect.DeepEqual(p.Settings.Files, []string{".env"}) {
		t.Errorf("Files = %#v, want %#v", p.Settings.Files, []string{".env"})
	}
	if got := p.Settings.Repos["api"]; got.Path != "repos/api" || got.DefaultBase != "main" {
		t.Errorf("Repos[api] = %#v, want path=repos/api defaultBase=main", got)
	}
}

func TestLoad_HasSchema(t *testing.T) {
	clearBecketConfig(t)

	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "schema present",
			body: `{"$schema":"x","repos":{},"branchPrefix":"feat/"}`,
			want: true,
		},
		{
			name: "schema absent",
			body: `{"repos":{},"branchPrefix":"feat/"}`,
			want: false,
		},
		{
			name: "schema present but empty string still counts as present",
			body: `{"$schema":"","repos":{},"branchPrefix":"feat/"}`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, tt.body)
			t.Chdir(root)

			p, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if p.HasSchema != tt.want {
				t.Errorf("HasSchema = %v, want %v", p.HasSchema, tt.want)
			}
		})
	}
}

func TestLoad_RepoOrderIsDocumentOrder(t *testing.T) {
	clearBecketConfig(t)

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "preserves declared order, not lexical",
			body: `{
  "repos": {
    "zeta": { "path": "z", "defaultBase": "main" },
    "alpha": { "path": "a", "defaultBase": "main" },
    "mike": { "path": "m", "defaultBase": "main" }
  },
  "branchPrefix": "feat/"
}`,
			want: []string{"zeta", "alpha", "mike"},
		},
		{
			name: "empty repos object",
			body: `{"repos":{},"branchPrefix":"feat/"}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, tt.body)
			t.Chdir(root)

			p, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(p.RepoOrder, tt.want) {
				t.Errorf("RepoOrder = %#v, want %#v", p.RepoOrder, tt.want)
			}
		})
	}
}

func TestLoad_NotFound(t *testing.T) {
	clearBecketConfig(t)

	dir := t.TempDir()
	t.Chdir(dir)

	_, err := Load()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestLoad_MissingFileFromEnv(t *testing.T) {
	// Find returns BECKET_CONFIG verbatim without stat-ing it; Load then fails
	// at os.ReadFile. The error is a filesystem error, distinct from ErrNotFound.
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist.json")
	t.Setenv("BECKET_CONFIG", missing)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want a read error for missing file")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("Load() error = ErrNotFound, want a filesystem read error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load() error = %v, want os.ErrNotExist", err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	clearBecketConfig(t)

	root := t.TempDir()
	writeConfig(t, root, `{ this is not json `)
	t.Chdir(root)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want a JSON syntax error")
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("Load() error = %v (%T), want *json.SyntaxError", err, err)
	}
}

func TestSettings_JSONShape(t *testing.T) {
	// Marshaling round-trips the documented key order ($schema, repos,
	// branchPrefix, files, docker, session) and applies omitempty to the
	// optional fields. The repos map marshals with keys sorted lexically by
	// encoding/json, independent of document order.
	tests := []struct {
		name string
		in   Settings
		want string
	}{
		{
			name: "all fields populated, ordered with sorted repo keys",
			in: Settings{
				Schema: "s",
				Repos: map[string]RepoConfig{
					"web": {Path: "repos/web", DefaultBase: "main"},
					"api": {Path: "repos/api", DefaultBase: "main"},
				},
				BranchPrefix: "feat/",
				Files:        []string{".env", ".env.local"},
				Docker:       "compose.yaml",
				Session:      "becket",
			},
			want: `{"$schema":"s","repos":{"api":{"path":"repos/api","defaultBase":"main"},"web":{"path":"repos/web","defaultBase":"main"}},"branchPrefix":"feat/","files":[".env",".env.local"],"docker":"compose.yaml","session":"becket"}`,
		},
		{
			name: "omitempty drops schema/files/docker/session; repos and branchPrefix always present",
			in: Settings{
				Repos:        map[string]RepoConfig{},
				BranchPrefix: "",
			},
			want: `{"repos":{},"branchPrefix":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

func TestRepoConfig_JSONShape(t *testing.T) {
	// Field order is path, defaultBase, setup, env, dev. path and defaultBase
	// are always emitted; setup, env and dev carry omitempty.
	tests := []struct {
		name string
		in   RepoConfig
		want string
	}{
		{
			name: "all fields populated in declared order",
			in: RepoConfig{
				Path:        "repos/api",
				DefaultBase: "main",
				Setup:       []string{"make deps", "make build"},
				Env:         map[string]string{"FOO": "bar"},
				Dev:         "make dev",
			},
			want: `{"path":"repos/api","defaultBase":"main","setup":["make deps","make build"],"env":{"FOO":"bar"},"dev":"make dev"}`,
		},
		{
			name: "omitempty drops setup/env/dev when zero",
			in: RepoConfig{
				Path:        "repos/api",
				DefaultBase: "main",
			},
			want: `{"path":"repos/api","defaultBase":"main"}`,
		},
		{
			name: "required fields emitted even when empty strings",
			in:   RepoConfig{},
			want: `{"path":"","defaultBase":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

func TestRepoConfig_RoundTrip(t *testing.T) {
	// Unmarshal -> Marshal should be stable for a fully-populated entry.
	const in = `{"path":"repos/api","defaultBase":"main","setup":["a"],"env":{"K":"V"},"dev":"d"}`
	var rc RepoConfig
	if err := json.Unmarshal([]byte(in), &rc); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	out, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	if string(out) != in {
		t.Errorf("round-trip =\n  %s\nwant\n  %s", out, in)
	}
}

// resolve evaluates symlinks so /var vs /private/var on macOS temp dirs do not
// cause spurious mismatches. Falls back to the original path if it cannot be
// resolved (e.g. it does not exist).
func resolve(t *testing.T, p string) string {
	t.Helper()
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
