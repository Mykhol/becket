package cli

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/render"
)

func newPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pr [id]",
		Short: "Open GitHub PRs for all repos (requires gh)",
		Args:  cobra.MaximumNArgs(1),
		Run:   func(_ *cobra.Command, args []string) { runPR(args) },
	}
}

func runPR(args []string) {
	if _, err := exec.LookPath("gh"); err != nil {
		render.Die("'gh' (GitHub CLI) not found. Install: https://cli.github.com")
	}
	p := loadPlatform()
	id := requireWorkspaceID(arg0(args), "Usage: becket pr [id] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)

	title := id
	if m.Description != "" {
		title = id + ": " + m.Description
	}

	ws := wsPath(p, id)
	for _, repo := range m.Order {
		wt := filepath.Join(ws, repo)
		branch := m.Repos[repo].Branch
		base := m.Repos[repo].Base
		render.Info("Creating PR for %s (%s → %s)...", repo, branch, base)
		cmd := exec.Command("gh", "pr", "create",
			"--title", title,
			"--body", "Workspace: `"+id+"`",
			"--base", base,
			"--head", branch)
		cmd.Dir = wt
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if cmd.Run() != nil {
			render.Warn("PR for %s may already exist — skipping.", repo)
		}
	}
}
