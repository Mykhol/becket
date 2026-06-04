package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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

// writeEmbeddedSchema writes one embedded schema file into dir (best-effort).
func writeEmbeddedSchema(dir, name string) {
	if data, err := schemas.ReadFile("schema/" + name); err == nil {
		_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
}
