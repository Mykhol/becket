package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/jsonfmt"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Update config + schemas to latest version",
		Args:  cobra.NoArgs,
		Run:   func(_ *cobra.Command, _ []string) { runUpgrade() },
	}
}

func runUpgrade() {
	p := loadPlatform()
	becketDir := filepath.Dir(p.ConfigFile)

	// ── settings.json ──────────────────────────────────────────────────────────
	writeEmbeddedSchema(becketDir, "settings.schema.json")
	const settingsRef = "./settings.schema.json"
	changed := false
	if !p.HasSchema || p.Settings.Schema != settingsRef {
		changed = true
	}
	p.Settings.Schema = settingsRef
	if p.Settings.BranchPrefix == "" {
		p.Settings.BranchPrefix = "feature/"
		changed = true
	}
	if changed {
		if data, err := jsonfmt.Indent2(p.Settings); err == nil {
			_ = os.WriteFile(p.ConfigFile, data, 0o644)
		}
		render.Info("Updated settings.json")
	} else {
		render.Info("settings.json is already current.")
	}

	// ── migrate legacy .becket/workspaces ────────────────────────────────────────
	migrateLegacyWorkspaces(p)

	// ── workspace manifests ──────────────────────────────────────────────────────
	const wsRef = "./workspace.schema.json"
	if fi, err := os.Stat(p.WorkspacesDir); err == nil && fi.IsDir() {
		for _, mp := range workspace.List(p.WorkspacesDir) {
			m, err := workspace.Load(mp)
			if err != nil {
				continue
			}
			writeEmbeddedSchema(filepath.Dir(mp), "workspace.schema.json")
			wchanged := m.Schema != wsRef
			m.Schema = wsRef
			if wchanged {
				_ = workspace.Save(mp, m)
				render.Info("Updated workspace: %s", m.ID)
			} else {
				render.Info("Workspace %s is already current.", m.ID)
			}
		}
	}

	// ── seed new platform files into existing workspaces ─────────────────────────
	if len(p.Settings.Files) > 0 {
		if fi, err := os.Stat(p.WorkspacesDir); err == nil && fi.IsDir() {
			seededAny := false
			for _, mp := range workspace.List(p.WorkspacesDir) {
				m, err := workspace.Load(mp)
				if err != nil {
					continue
				}
				ws := filepath.Dir(mp)
				for _, file := range p.Settings.Files {
					src := filepath.Join(p.Dir, file)
					dst := filepath.Join(ws, filepath.Base(file))
					if _, err := os.Stat(src); err != nil {
						continue
					}
					if _, err := os.Stat(dst); err != nil {
						if copyPath(src, dst) == nil {
							render.Info("Seeded %s → workspace %s", file, m.ID)
							seededAny = true
						}
					}
				}
			}
			if !seededAny {
				render.Info("Workspace files are already up to date.")
			}
		}
	}

	fmt.Println()
	render.Info("Upgrade complete.")
}

// migrateLegacyWorkspaces moves everything under the legacy .becket/workspaces
// dir into the configured workspaces dir and repairs each moved git worktree's
// link back from its main repo. Git records worktree locations by absolute
// path, so a bare rename leaves the main repo pointing at the old location;
// `git worktree repair` run inside the moved worktree fixes that back-pointer.
// Updates p.WorkspacesDir so the rest of the upgrade operates on the new
// location.
func migrateLegacyWorkspaces(p *config.Platform) {
	legacy := p.LegacyWorkspacesDir
	if legacy == "" {
		return
	}
	target := config.WorkspacesPath(p.Dir, p.Settings)
	if err := os.MkdirAll(target, 0o755); err != nil {
		render.Die("%v", err)
	}
	entries, err := os.ReadDir(legacy)
	if err != nil {
		render.Die("%v", err)
	}
	for _, e := range entries {
		src := filepath.Join(legacy, e.Name())
		dst := filepath.Join(target, e.Name())
		if _, err := os.Stat(dst); err == nil {
			render.Warn("Not migrating %s: %s already exists.", e.Name(), dst)
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			render.Warn("Could not move %s: %v", e.Name(), err)
			continue
		}
		render.Info("Moved %s → %s", e.Name(), dst)
		repairWorktrees(dst)
	}
	// Remove the legacy dir only once everything has moved out of it.
	if rest, err := os.ReadDir(legacy); err == nil && len(rest) == 0 {
		_ = os.Remove(legacy)
		p.LegacyWorkspacesDir = ""
	}
	p.WorkspacesDir = target
}

// repairWorktrees runs `git worktree repair` inside each repo checkout of a
// moved workspace (best-effort; a worktree's .git is a file, not a dir).
func repairWorktrees(ws string) {
	entries, err := os.ReadDir(ws)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wt := filepath.Join(ws, e.Name())
		if fi, err := os.Stat(filepath.Join(wt, ".git")); err != nil || fi.IsDir() {
			continue
		}
		if err := git.Quiet(wt, "worktree", "repair"); err != nil {
			render.Warn("Could not repair worktree %s: run 'git worktree repair' there manually.", wt)
		}
	}
}

// writeEmbeddedSchema writes one embedded schema file into dir (best-effort).
func writeEmbeddedSchema(dir, name string) {
	if data, err := schemas.ReadFile("schema/" + name); err == nil {
		_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
}
