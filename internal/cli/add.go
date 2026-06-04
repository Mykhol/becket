package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [id] <repo>",
		Short: "Add a repo to an existing workspace",
		Args:  cobra.RangeArgs(1, 2),
		Run:   func(_ *cobra.Command, args []string) { runAdd(args) },
	}
}

func runAdd(args []string) {
	var id, repo string
	switch len(args) {
	case 2:
		id, repo = args[0], args[1]
	case 1:
		detected, err := workspace.Detect()
		if err != nil {
			render.Die("Usage: becket add [id] <repo> (or run from inside a workspace)")
		}
		id, repo = detected, args[0]
	default:
		render.Die("Usage: becket add [id] <repo>")
	}

	p := loadPlatform()
	m := mustLoadManifest(p, id)

	if _, exists := m.Repos[repo]; exists {
		render.Die("Repo '%s' is already in workspace '%s'.", repo, id)
	}
	rc, ok := p.Settings.Repos[repo]
	if !ok {
		render.Die("Repo '%s' not found in platform config.", repo)
	}

	// Reuse the branch of the workspace's first repo.
	existingBranch := m.Repos[m.Order[0]].Branch
	base := rc.DefaultBase

	ws := wsPath(p, id)
	wt := filepath.Join(ws, repo)
	render.Info("Creating worktree: %s → %s (base: %s)", repo, existingBranch, base)
	gitOrExit(git.Run(repoAbsPath(p, rc.Path), "worktree", "add", wt, "-b", existingBranch, base))

	m.Repos[repo] = workspace.RepoEntry{Branch: existingBranch, Base: base}
	m.Order = append(m.Order, repo)
	mustSaveManifest(p, id, m)
	writeAgentsMD(ws, m, p.Settings)

	render.Info("Added %s to workspace %s", repo, id)
}
