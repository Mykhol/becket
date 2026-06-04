package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// indexOf returns the byte offset of sub in s, or -1 if absent.
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestManifestMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   Manifest
		want string
	}{
		{
			name: "full manifest with all optional fields",
			in: Manifest{
				Schema:      "https://example.com/schema.json",
				ID:          "ws-1",
				Created:     "2026-01-01T00:00:00Z",
				Description: "a workspace",
				StackParent: "ws-0",
				Repos: map[string]RepoEntry{
					"alpha": {Branch: "feat/a", Base: "main"},
				},
				Status: &Status{Text: "wip", UpdatedAt: "2026-01-02", UpdatedBy: "bob"},
				Order:  []string{"alpha"},
			},
			want: `{"$schema":"https://example.com/schema.json","id":"ws-1","created":"2026-01-01T00:00:00Z","description":"a workspace","stackParent":"ws-0","repos":{"alpha":{"branch":"feat/a","base":"main"}},"status":{"text":"wip","updatedAt":"2026-01-02","updatedBy":"bob"}}`,
		},
		{
			name: "omits schema, stackParent, and status when empty",
			in: Manifest{
				ID:          "ws-2",
				Created:     "2026-01-03",
				Description: "",
				Repos:       map[string]RepoEntry{},
				Order:       []string{},
			},
			want: `{"id":"ws-2","created":"2026-01-03","description":"","repos":{}}`,
		},
		{
			name: "repos emitted in Order, not map order",
			in: Manifest{
				ID:      "ws-3",
				Created: "2026-01-04",
				Repos: map[string]RepoEntry{
					"zeta":  {Branch: "z", Base: "main"},
					"alpha": {Branch: "a", Base: "main"},
					"mid":   {Branch: "m", Base: "main"},
				},
				// Insertion order deliberately not sorted to prove Order wins.
				Order: []string{"zeta", "alpha", "mid"},
			},
			want: `{"id":"ws-3","created":"2026-01-04","description":"","repos":{"zeta":{"branch":"z","base":"main"},"alpha":{"branch":"a","base":"main"},"mid":{"branch":"m","base":"main"}}}`,
		},
		{
			name: "falls back to sorted keys when Order is empty",
			in: Manifest{
				ID:      "ws-4",
				Created: "2026-01-05",
				Repos: map[string]RepoEntry{
					"zeta":  {Branch: "z", Base: "main"},
					"alpha": {Branch: "a", Base: "main"},
					"mid":   {Branch: "m", Base: "main"},
				},
				Order: nil,
			},
			want: `{"id":"ws-4","created":"2026-01-05","description":"","repos":{"alpha":{"branch":"a","base":"main"},"mid":{"branch":"m","base":"main"},"zeta":{"branch":"z","base":"main"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("json.Marshal returned error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal mismatch\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

// TestManifestMarshalTopLevelKeyOrder pins the relative order of the top-level
// keys independently of the exact content, so the test still guards order even
// if values change.
func TestManifestMarshalTopLevelKeyOrder(t *testing.T) {
	m := Manifest{
		Schema:      "s",
		ID:          "id",
		Created:     "c",
		Description: "d",
		StackParent: "p",
		Repos:       map[string]RepoEntry{"r": {Branch: "b", Base: "base"}},
		Status:      &Status{Text: "t", UpdatedAt: "ua", UpdatedBy: "ub"},
		Order:       []string{"r"},
	}
	got, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	s := string(got)
	wantOrder := []string{`"$schema"`, `"id"`, `"created"`, `"description"`, `"stackParent"`, `"repos"`, `"status"`}
	prev := -1
	for _, key := range wantOrder {
		at := indexOf(s, key)
		if at < 0 {
			t.Fatalf("key %s missing from output: %s", key, s)
		}
		if at <= prev {
			t.Errorf("key %s out of order (at %d, prev %d) in %s", key, at, prev, s)
		}
		prev = at
	}
}

func TestLoad(t *testing.T) {
	t.Run("round-trips fields and recovers repo Order from document order", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".becket.json")
		// Repos intentionally NOT in sorted order in the document.
		raw := `{
  "$schema": "https://example.com/s.json",
  "id": "ws-load",
  "created": "2026-02-01T00:00:00Z",
  "description": "loaded",
  "stackParent": "parent-ws",
  "repos": {
    "gamma": {"branch": "g", "base": "main"},
    "alpha": {"branch": "a", "base": "develop"},
    "beta": {"branch": "b", "base": "main"}
  },
  "status": {"text": "ok", "updatedAt": "2026-02-02", "updatedBy": "ann"}
}`
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		m, err := Load(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		if m.Schema != "https://example.com/s.json" {
			t.Errorf("Schema = %q", m.Schema)
		}
		if m.ID != "ws-load" {
			t.Errorf("ID = %q", m.ID)
		}
		if m.Created != "2026-02-01T00:00:00Z" {
			t.Errorf("Created = %q", m.Created)
		}
		if m.Description != "loaded" {
			t.Errorf("Description = %q", m.Description)
		}
		if m.StackParent != "parent-ws" {
			t.Errorf("StackParent = %q", m.StackParent)
		}
		if m.Status == nil || m.Status.Text != "ok" || m.Status.UpdatedAt != "2026-02-02" || m.Status.UpdatedBy != "ann" {
			t.Errorf("Status = %+v", m.Status)
		}
		wantRepos := map[string]RepoEntry{
			"gamma": {Branch: "g", Base: "main"},
			"alpha": {Branch: "a", Base: "develop"},
			"beta":  {Branch: "b", Base: "main"},
		}
		if !reflect.DeepEqual(m.Repos, wantRepos) {
			t.Errorf("Repos = %+v, want %+v", m.Repos, wantRepos)
		}
		wantOrder := []string{"gamma", "alpha", "beta"}
		if !reflect.DeepEqual(m.Order, wantOrder) {
			t.Errorf("Order = %v, want %v (document order)", m.Order, wantOrder)
		}
	})

	t.Run("propagates read error for missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected not-exist error, got %v", err)
		}
	})

	t.Run("propagates parse error for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".becket.json")
		if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".becket.json")
	m := &Manifest{
		Schema:      "https://example.com/s.json",
		ID:          "ws-save",
		Created:     "2026-03-01",
		Description: "saved",
		Repos: map[string]RepoEntry{
			"one": {Branch: "b1", Base: "main"},
			"two": {Branch: "b2", Base: "main"},
		},
		Order: []string{"one", "two"},
	}

	if err := Save(path, m); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	want := `{
  "$schema": "https://example.com/s.json",
  "id": "ws-save",
  "created": "2026-03-01",
  "description": "saved",
  "repos": {
    "one": {
      "branch": "b1",
      "base": "main"
    },
    "two": {
      "branch": "b2",
      "base": "main"
    }
  }
}
`
	if string(got) != want {
		t.Errorf("Save output mismatch\n got: %q\nwant: %q", got, want)
	}

	// Trailing newline must be present exactly once.
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Errorf("expected trailing newline, last byte = %q", got[len(got)-1:])
	}
	if len(got) >= 2 && got[len(got)-2] == '\n' {
		t.Errorf("expected exactly one trailing newline")
	}

	// 2-space indent: top-level keys are prefixed with exactly two spaces.
	if indexOf(string(got), "\n  \"id\"") < 0 {
		t.Errorf("expected 2-space indented top-level key, got: %s", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".becket.json")
	orig := &Manifest{
		Schema:      "https://example.com/s.json",
		ID:          "ws-rt",
		Created:     "2026-04-01",
		Description: "round trip",
		StackParent: "parent",
		Repos: map[string]RepoEntry{
			"zed":   {Branch: "z", Base: "main"},
			"apple": {Branch: "a", Base: "dev"},
		},
		Status: &Status{Text: "fine", UpdatedAt: "2026-04-02", UpdatedBy: "cara"},
		Order:  []string{"zed", "apple"},
	}

	if err := Save(path, orig); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if !reflect.DeepEqual(loaded, orig) {
		t.Errorf("round trip mismatch\n got: %+v\nwant: %+v", loaded, orig)
	}

	// Re-marshaling the loaded manifest must be byte-identical to the saved file.
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	if err := Save(path, loaded); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("re-save not stable\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestDetect(t *testing.T) {
	t.Run("finds id walking up from a nested subdirectory", func(t *testing.T) {
		root := t.TempDir()
		manifest := filepath.Join(root, ".becket.json")
		if err := os.WriteFile(manifest, []byte(`{"id":"ws-detect"}`), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		nested := filepath.Join(root, "a", "b", "c")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("mkdir nested: %v", err)
		}
		t.Chdir(nested)

		id, err := Detect()
		if err != nil {
			t.Fatalf("Detect error: %v", err)
		}
		if id != "ws-detect" {
			t.Errorf("Detect id = %q, want %q", id, "ws-detect")
		}
	})

	t.Run("returns ErrNotInWorkspace when no manifest is found", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		_, err := Detect()
		if err != ErrNotInWorkspace {
			t.Errorf("Detect error = %v, want ErrNotInWorkspace", err)
		}
	})

	t.Run("skips manifest with empty id and keeps walking up", func(t *testing.T) {
		root := t.TempDir()
		// Top-level manifest has the id.
		if err := os.WriteFile(filepath.Join(root, ".becket.json"), []byte(`{"id":"top-ws"}`), 0o644); err != nil {
			t.Fatalf("write top manifest: %v", err)
		}
		sub := filepath.Join(root, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("mkdir sub: %v", err)
		}
		// Inner manifest is present but has an empty id, so Detect must walk past it.
		if err := os.WriteFile(filepath.Join(sub, ".becket.json"), []byte(`{"id":""}`), 0o644); err != nil {
			t.Fatalf("write sub manifest: %v", err)
		}
		t.Chdir(sub)

		id, err := Detect()
		if err != nil {
			t.Fatalf("Detect error: %v", err)
		}
		if id != "top-ws" {
			t.Errorf("Detect id = %q, want %q", id, "top-ws")
		}
	})

	t.Run("skips manifest with invalid JSON and keeps walking up", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".becket.json"), []byte(`{"id":"good-ws"}`), 0o644); err != nil {
			t.Fatalf("write top manifest: %v", err)
		}
		sub := filepath.Join(root, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("mkdir sub: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sub, ".becket.json"), []byte(`{bad`), 0o644); err != nil {
			t.Fatalf("write sub manifest: %v", err)
		}
		t.Chdir(sub)

		id, err := Detect()
		if err != nil {
			t.Fatalf("Detect error: %v", err)
		}
		if id != "good-ws" {
			t.Errorf("Detect id = %q, want %q", id, "good-ws")
		}
	})
}

func TestRequire(t *testing.T) {
	t.Run("returns provided id without touching CWD", func(t *testing.T) {
		// Isolate in an empty dir so a fallback Detect would fail; the explicit
		// id must short-circuit before any detection.
		t.Chdir(t.TempDir())
		got, err := Require("explicit-id")
		if err != nil {
			t.Fatalf("Require error: %v", err)
		}
		if got != "explicit-id" {
			t.Errorf("Require id = %q, want %q", got, "explicit-id")
		}
	})

	t.Run("falls back to Detect when id is empty", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".becket.json"), []byte(`{"id":"detected-ws"}`), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		t.Chdir(root)
		got, err := Require("")
		if err != nil {
			t.Fatalf("Require error: %v", err)
		}
		if got != "detected-ws" {
			t.Errorf("Require id = %q, want %q", got, "detected-ws")
		}
	})

	t.Run("propagates ErrNotInWorkspace when id empty and none detected", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := Require("")
		if err != ErrNotInWorkspace {
			t.Errorf("Require error = %v, want ErrNotInWorkspace", err)
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns sorted manifest paths", func(t *testing.T) {
		wsDir := t.TempDir()
		// Create out of order to prove List sorts.
		names := []string{"charlie", "alpha", "bravo"}
		for _, n := range names {
			d := filepath.Join(wsDir, n)
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", n, err)
			}
			if err := os.WriteFile(filepath.Join(d, ".becket.json"), []byte(`{"id":"`+n+`"}`), 0o644); err != nil {
				t.Fatalf("write %s manifest: %v", n, err)
			}
		}
		// A workspace dir without a manifest must be excluded by the glob.
		if err := os.MkdirAll(filepath.Join(wsDir, "no-manifest"), 0o755); err != nil {
			t.Fatalf("mkdir no-manifest: %v", err)
		}

		got := List(wsDir)
		want := []string{
			filepath.Join(wsDir, "alpha", ".becket.json"),
			filepath.Join(wsDir, "bravo", ".becket.json"),
			filepath.Join(wsDir, "charlie", ".becket.json"),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("List = %v, want %v", got, want)
		}
	})

	t.Run("returns empty for a directory with no workspaces", func(t *testing.T) {
		got := List(t.TempDir())
		if len(got) != 0 {
			t.Errorf("List = %v, want empty", got)
		}
	})
}

// TestRepoEntryJSONFieldOrder pins RepoEntry's on-disk key order (branch, base).
func TestRepoEntryJSONFieldOrder(t *testing.T) {
	got, err := json.Marshal(RepoEntry{Branch: "feat/x", Base: "main"})
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	want := `{"branch":"feat/x","base":"main"}`
	if string(got) != want {
		t.Errorf("RepoEntry JSON = %s, want %s", got, want)
	}
}

// TestStatusJSONFieldOrder pins Status's on-disk key order (text, updatedAt, updatedBy).
func TestStatusJSONFieldOrder(t *testing.T) {
	got, err := json.Marshal(Status{Text: "hi", UpdatedAt: "2026-05-01", UpdatedBy: "dan"})
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	want := `{"text":"hi","updatedAt":"2026-05-01","updatedBy":"dan"}`
	if string(got) != want {
		t.Errorf("Status JSON = %s, want %s", got, want)
	}
}
