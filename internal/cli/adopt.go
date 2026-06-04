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

const adoptUsage = "Usage: becket adopt <id> [--repos r1,r2] [--base BRANCH] [--setup]"

func newAdoptCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "adopt <id> [options]",
		Short:              "Adopt existing branches into a new workspace",
		DisableFlagParsing: true,
		Run:                func(_ *cobra.Command, args []string) { runAdopt(args) },
	}
}

func runAdopt(args []string) {
	var baseOverride, reposFlag, id string
	runSetup := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--base":
			baseOverride, i = next(args, i), i+1
		case "--repos":
			reposFlag, i = next(args, i), i+1
		case "--setup":
			runSetup = true
		default:
			if len(a) > 0 && a[0] == '-' {
				render.Die("Unknown option: %s", a)
			}
			id = a
		}
	}
	if id == "" {
		render.Die("%s", adoptUsage)
	}

	p := loadPlatform()
	ws := wsPath(p, id)
	if fi, err := os.Stat(ws); err == nil && fi.IsDir() {
		render.Die("Workspace '%s' already exists at %s", id, ws)
	}

	repos := selectRepos(p, reposFlag)
	if len(repos) == 0 {
		render.Die("No repos selected.")
	}

	// Detect each repo's current branch and validate.
	detected := make(map[string]string, len(repos))
	for _, repo := range repos {
		rc, ok := p.Settings.Repos[repo]
		if !ok {
			render.Die("Repo '%s' not found in config.", repo)
		}
		repoAbs := repoAbsPath(p, rc.Path)
		cur, err := git.Output(repoAbs, "branch", "--show-current")
		if err != nil {
			render.Die("Could not read branch for '%s'.", repo)
		}
		if cur == "" {
			render.Die("Repo '%s' is in detached HEAD state.", repo)
		}
		base := baseOverride
		if base == "" {
			base = rc.DefaultBase
		}
		if cur == base {
			render.Die("Repo '%s' is already on base branch '%s'. Nothing to adopt.", repo, base)
		}
		detected[repo] = cur
		render.Info("Detected %s on branch %s", repo, cur)
	}

	if err := os.MkdirAll(filepath.Join(ws, "docs"), 0o755); err != nil {
		render.Die("%v", err)
	}
	render.Info("Created workspace: %s", ws)

	m := &workspace.Manifest{ID: id, Created: today(), Description: "", Repos: map[string]workspace.RepoEntry{}}
	for _, repo := range repos {
		rc := p.Settings.Repos[repo]
		repoAbs := repoAbsPath(p, rc.Path)
		base := baseOverride
		if base == "" {
			base = rc.DefaultBase
		}
		branch := detected[repo]
		wt := filepath.Join(ws, repo)

		stashed := false
		if !git.IsClean(repoAbs) {
			render.Warn("Stashing uncommitted changes in %s...", repo)
			gitOrExit(git.Run(repoAbs, "stash", "push", "-m", "becket adopt "+id))
			stashed = true
		}

		render.Info("Switching %s main dir → %s...", repo, base)
		gitOrExit(git.Run(repoAbs, "checkout", base))

		render.Info("Creating worktree: %s → %s", repo, branch)
		gitOrExit(git.Run(repoAbs, "worktree", "add", wt, branch))

		if stashed {
			render.Info("Restoring stashed changes into workspace %s...", repo)
			gitOrExit(git.Run(wt, "stash", "pop"))
		}

		m.Repos[repo] = workspace.RepoEntry{Branch: branch, Base: base}
		m.Order = append(m.Order, repo)
	}

	m.Schema = writeWorkspaceSchema(ws)
	mustSaveManifest(p, id, m)
	writeAgentsMD(ws, m, p.Settings)
	copyPlatformFiles(p, ws)

	fmt.Println()
	render.Info("Workspace %s ready at %s", id, ws)
	render.Info("Adopted branches:")
	for _, repo := range repos {
		render.Info("  %s → %s", repo, detected[repo])
	}

	if runSetup {
		fmt.Println()
		runSetupForWorkspace(p, id)
	}
}
