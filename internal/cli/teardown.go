package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

func newTeardownCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "teardown [id] [options]",
		Short:              "Remove worktrees + workspace",
		DisableFlagParsing: true,
		Run:                func(_ *cobra.Command, args []string) { runTeardown(args) },
	}
}

func runTeardown(args []string) {
	id := ""
	deleteBranches := false
	for _, a := range args {
		switch {
		case a == "--delete-branches":
			deleteBranches = true
		case len(a) > 0 && a[0] == '-':
			render.Die("Unknown option: %s", a)
		default:
			id = a
		}
	}
	if id == "" {
		detected, err := workspace.Detect()
		if err != nil {
			render.Die("Usage: becket teardown [id] [--delete-branches] (or run from inside a workspace)")
		}
		id = detected
	}

	p := loadPlatform()
	ws := wsPath(p, id)
	m, err := workspace.Load(manifestPath(p, id))
	if err != nil {
		render.Die("Workspace '%s' not found.", id)
	}

	for _, repo := range m.Order {
		rc, ok := p.Settings.Repos[repo]
		if !ok {
			render.Warn("Repo '%s' not in platform config, skipping worktree removal.", repo)
			continue
		}
		repoAbs := repoAbsPath(p, rc.Path)
		wt := filepath.Join(ws, repo)

		if fi, err := os.Stat(wt); err == nil && fi.IsDir() {
			render.Info("Removing worktree: %s", repo)
			if git.RunStdout(repoAbs, "worktree", "remove", wt, "--force") != nil {
				render.Warn("Could not remove worktree for %s", repo)
			}
		}
		if deleteBranches {
			branch := m.Repos[repo].Branch
			render.Info("Deleting branch: %s from %s", branch, repo)
			if git.RunStdout(repoAbs, "branch", "-D", branch) != nil {
				render.Warn("Could not delete branch %s", branch)
			}
		}
		_ = git.Quiet(repoAbs, "worktree", "prune")
	}

	if err := os.RemoveAll(ws); err != nil {
		render.Die("%v", err)
	}
	render.Info("Removed workspace: %s", id)
}
