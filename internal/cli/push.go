package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
)

func newPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push [id]",
		Short: "Push all repo branches to origin",
		Args:  cobra.MaximumNArgs(1),
		Run:   func(_ *cobra.Command, args []string) { runPush(args) },
	}
}

func runPush(args []string) {
	p := loadPlatform()
	id := requireWorkspaceID(arg0(args), "Usage: becket push [id] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)

	ws := wsPath(p, id)
	for _, repo := range m.Order {
		wt := filepath.Join(ws, repo)
		branch := m.Repos[repo].Branch
		render.Info("Pushing %s → origin/%s...", repo, branch)
		gitOrExit(git.Run(wt, "push", "-u", "origin", branch))
	}
	fmt.Println()
	render.Info("Push complete.")
}
