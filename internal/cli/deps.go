package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/jsonfmt"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

const depsUsage = "Usage: becket deps [id] [--repo NAME] [--force] (or run from inside a workspace)"

func newDepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deps [id] [options]",
		Short: "Install a workspace's dependencies",
		Long: `Run each selected repo's canonical dependency install (the 'deps.run'
commands), skipping repos whose lockfiles haven't changed since the last
install.

Options:
  --repo NAME   Install only this repo (default: the repo containing the
                current directory, or every repo in the workspace)
  --force       Reinstall even when the stamp is up to date`,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if helpRequested(cmd, args) {
				return
			}
			runDeps(args)
		},
	}
}

func runDeps(args []string) {
	var id, repoFlag string
	force := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--repo":
			repoFlag, i = next(args, i), i+1
		case "--force":
			force = true
		default:
			if len(a) > 0 && a[0] == '-' {
				render.Die("Unknown option: %s", a)
			}
			id = a
		}
	}
	id = requireWorkspaceID(id, depsUsage)

	p := loadPlatform()
	m := mustLoadManifest(p, id)
	ws := wsPath(p, id)

	if repoFlag != "" {
		if _, ok := m.Repos[repoFlag]; !ok {
			render.Die("Repo '%s' is not part of workspace '%s'.", repoFlag, id)
		}
	}
	repos := reposForDeps(ws, m, repoFlag)

	failed := false
	for _, repo := range repos {
		rc := p.Settings.Repos[repo]
		if rc.Deps == nil {
			render.Info("No deps configured for %s, skipping.", repo)
			continue
		}
		wt := filepath.Join(ws, repo)
		if fi, err := os.Stat(wt); err != nil || !fi.IsDir() {
			render.Warn("Worktree missing for %s, skipping.", repo)
			continue
		}
		if err := installRepoDeps(ws, repo, rc.Deps, wt, buildEnvPrefix(rc.Env), force); err != nil {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// reposForDeps resolves which repos 'becket deps' targets: an explicit
// --repo, else the repo the current directory is inside, else every repo in
// the workspace (manifest order).
func reposForDeps(ws string, m *workspace.Manifest, repoFlag string) []string {
	if repoFlag != "" {
		return []string{repoFlag}
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, repo := range m.Order {
			if pathContains(filepath.Join(ws, repo), cwd) {
				return []string{repo}
			}
		}
	}
	return append([]string(nil), m.Order...)
}

// depsStamp is the on-disk marker recording the last successful install.
type depsStamp struct {
	Key         string `json:"key"`
	InstalledAt string `json:"installedAt"`
}

// depsStampPath is kept outside the git worktree (under the workspace's own
// .becket dir) so installing dependencies never dirties git status.
func depsStampPath(ws, repo string) string {
	return filepath.Join(ws, ".becket", "deps", repo+".json")
}

// depsKey hashes the run commands and current lockfile contents, so either
// changing invalidates a previous install's stamp.
func depsKey(wt string, dc *config.DepsConfig) string {
	h := sha256.New()
	for _, cmd := range dc.Run {
		fmt.Fprintf(h, "run:%s\x00", cmd)
	}
	for _, lf := range dc.Lockfiles {
		data, err := os.ReadFile(filepath.Join(wt, lf))
		if err != nil {
			fmt.Fprintf(h, "lockfile:%s\x00<missing>\x00", lf)
			continue
		}
		fmt.Fprintf(h, "lockfile:%s\x00%s\x00", lf, data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// hasGlobMeta reports whether s contains glob metacharacters.
func hasGlobMeta(s string) bool { return strings.ContainsAny(s, "*?[") }

// depsUpToDate reports whether repo's stamp matches the current key and every
// non-glob Dirs entry still exists (a glob install dir can't be checked for
// existence without running the glob, which the caller does separately at
// prune time; the stamp key is what gc/deps rely on there).
func depsUpToDate(ws, repo, wt string, dc *config.DepsConfig) bool {
	raw, err := os.ReadFile(depsStampPath(ws, repo))
	if err != nil {
		return false
	}
	var stamp depsStamp
	if json.Unmarshal(raw, &stamp) != nil {
		return false
	}
	if stamp.Key != depsKey(wt, dc) {
		return false
	}
	for _, d := range dc.Dirs {
		if hasGlobMeta(d) {
			continue
		}
		if _, err := os.Stat(filepath.Join(wt, d)); err != nil {
			return false
		}
	}
	return true
}

// installRepoDeps installs repo's dependencies when the stamp is stale or
// force is set, streaming each command's output, and writes a fresh stamp on
// success. Returns an error only when a run command failed (already warned).
func installRepoDeps(ws, repo string, dc *config.DepsConfig, wt, envPrefix string, force bool) error {
	if !force && depsUpToDate(ws, repo, wt, dc) {
		render.Info("%s dependencies up to date.", repo)
		return nil
	}
	render.Info("Installing deps for %s...", repo)
	for _, cmd := range dc.Run {
		render.Info("  Running: %s", cmd)
		if err := runShell(wt, envPrefix+cmd); err != nil {
			render.Warn("  Command failed: %s", cmd)
			return err
		}
	}
	stamp := depsStamp{Key: depsKey(wt, dc), InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	data, err := jsonfmt.Indent2(stamp)
	if err != nil {
		return nil
	}
	stampPath := depsStampPath(ws, repo)
	if err := os.MkdirAll(filepath.Dir(stampPath), 0o755); err != nil {
		return nil
	}
	_ = os.WriteFile(stampPath, data, 0o644)
	return nil
}
