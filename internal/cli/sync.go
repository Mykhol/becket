package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [id]",
		Short: "Rebase all repos against their base branch",
		Args:  cobra.MaximumNArgs(1),
		Run:   func(_ *cobra.Command, args []string) { runSync(args) },
	}
}

func runSync(args []string) {
	p := loadPlatform()
	id := requireWorkspaceID(arg0(args), "Usage: becket sync [id] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)

	if m.StackParent != "" {
		render.Warn("Workspace '%s' is stacked on '%s'. Use becket restack %s instead of sync.", id, m.StackParent, id)
		os.Exit(1)
	}

	ws := wsPath(p, id)
	for _, repo := range m.Order {
		wt := filepath.Join(ws, repo)
		base := m.Repos[repo].Base
		render.Info("Syncing %s ← origin/%s...", repo, base)
		gitOrExit(git.Run(wt, "fetch", "origin", base))
		gitOrExit(git.Run(wt, "rebase", "origin/"+base))
		removePycacheOrphans(wt)
	}
	fmt.Println()
	render.Info("Sync complete.")
}
