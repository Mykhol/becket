// Package config models becket's platform settings (.becket/settings.json) and
// its discovery (walk up from CWD), matching the bash find_config/load_config.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/Mykhol/becket/internal/jsonfmt"
)

// RepoConfig is one entry under settings.repos. Field order matches the JSON
// the bash script writes (path, defaultBase, …); omitempty keeps optional keys
// absent unless set, as the original does.
type RepoConfig struct {
	Path        string            `json:"path"`
	DefaultBase string            `json:"defaultBase"`
	Setup       []string          `json:"setup,omitempty"`
	Deps        *DepsConfig       `json:"deps,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Dev         string            `json:"dev,omitempty"`
}

// DepsConfig is a repo's canonical, idempotent dependency install, run by
// 'becket deps' and (unforced) before a repo's setup commands.
type DepsConfig struct {
	// Run is the shell commands that install dependencies, run in the
	// worktree with the repo's env.
	Run []string `json:"run"`
	// Lockfiles are paths, relative to the worktree, whose contents key the
	// install: a changed lockfile invalidates the stamp.
	Lockfiles []string `json:"lockfiles,omitempty"`
	// Dirs are dependency directories, relative to the worktree, that 'becket
	// gc' may prune once idle (path.Match-style globs allowed, e.g.
	// "apps/*/node_modules").
	Dirs []string `json:"dirs,omitempty"`
}

// Settings is the platform config. Struct field order is the on-disk key order
// ($schema, repos, branchPrefix, …); the repos map marshals with keys sorted
// lexically by encoding/json.
type Settings struct {
	Schema        string                `json:"$schema,omitempty"`
	Repos         map[string]RepoConfig `json:"repos"`
	BranchPrefix  string                `json:"branchPrefix"`
	WorkspacesDir string                `json:"workspacesDir,omitempty"`
	Files         []string              `json:"files,omitempty"`
	Docker        string                `json:"docker,omitempty"`
	Session       string                `json:"session,omitempty"`
	GC            *GCConfig             `json:"gc,omitempty"`
}

// GCConfig tunes 'becket gc': which workspace ids are treated as throwaway,
// and the idle thresholds for removal and dependency pruning. Nil fields fall
// back to their defaults (see the gc command).
type GCConfig struct {
	// Disposable is a set of workspace-id globs (path.Match) treated as
	// throwaway. Defaults to ["spike-*", "*review*"].
	Disposable []string `json:"disposable,omitempty"`
	// IdleDays is how many days idle before a disposable or no-work workspace
	// is eligible for removal. Defaults to 3.
	IdleDays *int `json:"idleDays,omitempty"`
	// DepsIdleDays is how many days idle before a kept workspace's dependency
	// directories are pruned. Defaults to 2.
	DepsIdleDays *int `json:"depsIdleDays,omitempty"`
	// Archive lists workspace-root entries copied to
	// <platform>/.becket/archive/<id>/ before gc removes a workspace, so
	// reports and notes outlive it. Defaults to [".reports", "docs"].
	Archive []string `json:"archive,omitempty"`
}

// Platform is a loaded config plus the derived paths the commands operate on.
type Platform struct {
	ConfigFile    string
	Dir           string // platform root (parent of .becket)
	WorkspacesDir string
	// LegacyWorkspacesDir is set when workspaces still live under the pre-1.x
	// .becket/workspaces location and that isn't the configured target. While
	// the target dir doesn't exist yet, WorkspacesDir points here so commands
	// keep working until 'becket upgrade' migrates.
	LegacyWorkspacesDir string
	HasSchema           bool     // whether the on-disk config carried a $schema key
	RepoOrder           []string // repo names in config document order
	Settings            Settings
}

// DefaultWorkspacesRel is where workspaces live relative to the platform root
// when settings.workspacesDir is unset.
const DefaultWorkspacesRel = "workspaces"

// WorkspacesPath resolves the configured (or default) workspaces directory for
// a platform rooted at dir — the migration target, ignoring any legacy layout.
func WorkspacesPath(dir string, s Settings) string {
	rel := s.WorkspacesDir
	if rel == "" {
		rel = DefaultWorkspacesRel
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(dir, rel)
}

// ErrNotFound is returned by Find/Load when no platform config is discoverable.
var ErrNotFound = errors.New("no .becket/settings.json found")

// Find returns the path to settings.json, honoring $BECKET_CONFIG or walking up
// from the current directory, matching the bash discovery order.
func Find() (string, error) {
	if env := os.Getenv("BECKET_CONFIG"); env != "" {
		return env, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ".becket", "settings.json")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// Load discovers and parses the platform config, deriving its paths.
func Load() (*Platform, error) {
	path, err := Find()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	// Detect $schema presence on the raw object (load_config's outdated check).
	var probe map[string]json.RawMessage
	_ = json.Unmarshal(raw, &probe)
	_, hasSchema := probe["$schema"]

	dir := filepath.Dir(filepath.Dir(path)) // <platform>/.becket/settings.json -> <platform>
	wsDir, legacy := deriveWorkspacesDirs(dir, s)
	return &Platform{
		ConfigFile:          path,
		Dir:                 dir,
		WorkspacesDir:       wsDir,
		LegacyWorkspacesDir: legacy,
		HasSchema:           hasSchema,
		RepoOrder:           jsonfmt.NestedKeyOrder(raw, "repos"),
		Settings:            s,
	}, nil
}

// deriveWorkspacesDirs picks the effective workspaces dir plus the legacy
// .becket/workspaces path when one exists and isn't the configured target
// (i.e. a migration is pending). With no explicit workspacesDir setting and no
// migrated target yet, the legacy dir stays effective so existing platforms
// keep working until 'becket upgrade' moves them.
func deriveWorkspacesDirs(dir string, s Settings) (wsDir, legacy string) {
	wsDir = WorkspacesPath(dir, s)
	legacyDir := filepath.Join(dir, ".becket", "workspaces")
	if wsDir == legacyDir {
		return wsDir, "" // explicitly configured to the old location: not legacy
	}
	if fi, err := os.Stat(legacyDir); err != nil || !fi.IsDir() {
		return wsDir, ""
	}
	if s.WorkspacesDir == "" {
		if fi, err := os.Stat(wsDir); err != nil || !fi.IsDir() {
			wsDir = legacyDir
		}
	}
	return wsDir, legacyDir
}
