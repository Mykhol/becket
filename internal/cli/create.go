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

const createUsage = "Usage: becket create <id> [--desc TEXT] [--repos r1,r2] [--base BRANCH] [--stacked-on PARENT_ID] [--setup]"

func newCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <id> [options]",
		Short: "Create workspace + worktrees",
		Long: `Create a workspace and a git worktree per selected repo, all on a new
shared feature branch.

Options:
  --desc TEXT       Description (slugified into the branch name)
  --repos r1,r2     Repos to include (default: all configured repos)
  --base BRANCH     Override the base branch for all repos
  --stacked-on ID   Stack on another workspace (its branches become the base)
  --setup           Run setup commands after creating`,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if helpRequested(cmd, args) {
				return
			}
			runCreate(args)
		},
	}
}

func runCreate(args []string) {
	var desc, baseOverride, reposFlag, stackParent, id string
	runSetup := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--desc":
			desc, i = next(args, i), i+1
		case "--base":
			baseOverride, i = next(args, i), i+1
		case "--repos":
			reposFlag, i = next(args, i), i+1
		case "--setup":
			runSetup = true
		case "--stacked-on":
			stackParent, i = next(args, i), i+1
		default:
			if len(a) > 0 && a[0] == '-' {
				render.Die("Unknown option: %s", a)
			}
			id = a
		}
	}
	if id == "" {
		render.Die("%s", createUsage)
	}

	p := loadPlatform()

	var parent *workspace.Manifest
	if stackParent != "" {
		pm := manifestPath(p, stackParent)
		if _, err := os.Stat(pm); err != nil {
			render.Die("Stack parent '%s' not found (expected %s).", stackParent, pm)
		}
		if baseOverride != "" {
			render.Die("Cannot combine --base with --stacked-on (the parent's branches are the base).")
		}
		parent, _ = workspace.Load(pm)
	}

	ws := wsPath(p, id)
	if fi, err := os.Stat(ws); err == nil && fi.IsDir() {
		render.Die("Workspace '%s' already exists at %s", id, ws)
	}

	branchSuffix := id
	if desc != "" {
		branchSuffix = id + "-" + slugify(desc)
	}
	branch := p.Settings.BranchPrefix + branchSuffix

	repos := selectRepos(p, reposFlag)
	if len(repos) == 0 {
		render.Die("No repos selected.")
	}
	for _, repo := range repos {
		if _, ok := p.Settings.Repos[repo]; !ok {
			render.Die("Repo '%s' not found in config.", repo)
		}
	}

	if err := os.MkdirAll(filepath.Join(ws, "docs"), 0o755); err != nil {
		render.Die("%v", err)
	}
	render.Info("Created workspace: %s", ws)

	m := &workspace.Manifest{
		ID: id, Created: today(), Description: desc, StackParent: stackParent,
		Repos: map[string]workspace.RepoEntry{},
	}
	for _, repo := range repos {
		rc := p.Settings.Repos[repo]
		repoAbs := repoAbsPath(p, rc.Path)

		var base string
		switch {
		case stackParent != "":
			if parent != nil {
				if pe, ok := parent.Repos[repo]; ok {
					base = pe.Branch
				}
			}
			if base == "" {
				base = rc.DefaultBase
				render.Warn("Stack parent '%s' has no '%s' — basing on '%s' instead.", stackParent, repo, base)
			}
		case baseOverride != "":
			base = baseOverride
		default:
			base = rc.DefaultBase
		}

		start := base
		if stackParent == "" {
			start = fetchedBase(repoAbs, repo, base)
		}

		wt := filepath.Join(ws, repo)
		render.Info("Creating worktree: %s → %s (base: %s)", repo, branch, start)
		gitOrExit(git.Run(repoAbs, "worktree", "add", "--no-track", wt, "-b", branch, start))

		m.Repos[repo] = workspace.RepoEntry{Branch: branch, Base: base}
		m.Order = append(m.Order, repo)
	}

	m.Schema = writeWorkspaceSchema(ws)
	mustSaveManifest(p, id, m)
	writeAgentsMD(ws, m, p.Settings)
	copyPlatformFiles(p, ws)

	fmt.Println()
	render.Info("Workspace %s ready at %s", id, ws)
	render.Info("Branch: %s", branch)

	if runSetup {
		fmt.Println()
		runSetupForWorkspace(p, id)
	}
}

// fetchedBase resolves the start point for a new branch: origin/<base> after a
// fetch, falling back to the local ref when the repo has no origin or the
// fetch fails. Branching from a stale local base is the silent failure this
// guards against — the branch starts behind origin from its first commit.
func fetchedBase(repoAbs, repo, base string) string {
	if git.Quiet(repoAbs, "remote", "get-url", "origin") != nil {
		return base
	}
	if git.Quiet(repoAbs, "fetch", "origin", base) != nil {
		render.Warn("Could not fetch origin/%s for %s — branching from local '%s'.", base, repo, base)
		return base
	}
	if !git.Verify(repoAbs, "origin/"+base) {
		return base
	}
	return "origin/" + base
}

// next returns args[i+1] or "" if out of range (mirrors bash `shift 2`).
func next(args []string, i int) string {
	if i+1 < len(args) {
		return args[i+1]
	}
	return ""
}
