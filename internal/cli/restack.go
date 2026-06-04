package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

func newRestackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restack [id]",
		Short: "Rebase a stacked workspace onto its parent's current tips",
		Args:  cobra.MaximumNArgs(1),
		Run:   func(_ *cobra.Command, args []string) { runRestack(args) },
	}
}

func runRestack(args []string) {
	p := loadPlatform()
	id := requireWorkspaceID(arg0(args), "Usage: becket restack [id] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)

	if m.StackParent == "" {
		render.Die("Workspace '%s' has no stackParent. Set it in .becket.json, or recreate with --stacked-on.", id)
	}
	parentPath := manifestPath(p, m.StackParent)
	parent, err := workspace.Load(parentPath)
	if err != nil {
		render.Die("Stack parent '%s' not found (expected %s).", m.StackParent, parentPath)
	}

	render.Info("Restacking %s onto %s...", id, m.StackParent)

	ws := wsPath(p, id)
	anyConflict := false
	for _, repo := range m.Order {
		wt := filepath.Join(ws, repo)
		if fi, err := os.Stat(wt); err != nil || !fi.IsDir() {
			render.Warn("Skipping %s: worktree missing at %s.", repo, wt)
			continue
		}
		pe, ok := parent.Repos[repo]
		if !ok || pe.Branch == "" {
			render.Info("Skipping %s: parent '%s' has no worktree for this repo.", repo, m.StackParent)
			continue
		}
		parentBranch := pe.Branch

		if !git.IsClean(wt) {
			render.Warn("Skipping %s: working tree dirty. Commit or stash first.", repo)
			anyConflict = true
			continue
		}

		render.Info("Fetching %s ← origin/%s...", repo, parentBranch)
		if git.Run(wt, "fetch", "origin", parentBranch) != nil {
			render.Warn("Fetch failed in %s.", repo)
			anyConflict = true
			continue
		}

		// Record the parent's branch as the new base (the parent may have moved).
		entry := m.Repos[repo]
		entry.Base = parentBranch
		m.Repos[repo] = entry
		mustSaveManifest(p, id, m)

		render.Info("Rebasing %s → origin/%s...", repo, parentBranch)
		if git.Run(wt, "rebase", "origin/"+parentBranch) != nil {
			render.Warn("Rebase conflict in %s:", repo)
			render.Warn("  cd %s", wt)
			render.Warn("  # resolve conflicts, then:  git add . && git rebase --continue")
			render.Warn("  # or to bail:                git rebase --abort")
			anyConflict = true
		}
	}

	fmt.Println()
	if anyConflict {
		render.Warn("Restack finished with issues — see warnings above.")
		os.Exit(1)
	}
	render.Info("Restack complete. If branches are already on origin, push with: git push --force-with-lease")
}
