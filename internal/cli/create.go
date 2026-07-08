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

const createUsage = "Usage: becket create <id> [--desc TEXT] [--repos r1,r2] [--base BRANCH] [--branch BRANCH] [--stacked-on PARENT_ID] [--setup]"

func newCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <id> [options]",
		Short: "Create workspace + worktrees",
		Long: `Create a workspace and a git worktree per selected repo, all on a
shared feature branch.

Options:
  --desc TEXT       Description (slugified into the branch name)
  --repos r1,r2     Repos to include (default: all configured repos)
  --base BRANCH     Override the base branch for all repos
  --branch BRANCH   Use an existing branch instead of creating a new one
                    (fetches it from origin if needed)
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
	var desc, baseOverride, reposFlag, stackParent, branchFlag, id string
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
		case "--branch":
			branchFlag, i = next(args, i), i+1
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

	// --branch uses an existing branch verbatim, so it can't combine with
	// options that derive or base a fresh branch.
	if branchFlag != "" {
		if desc != "" {
			render.Die("Cannot combine --branch with --desc (the branch already exists).")
		}
		if stackParent != "" {
			render.Die("Cannot combine --branch with --stacked-on (the branch already exists).")
		}
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
	if branchFlag != "" {
		// --branch uses an existing branch verbatim; no prefix, no slug.
		branch = branchFlag
	}

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

		wt := filepath.Join(ws, repo)
		if branchFlag != "" {
			// Best-effort fetch of the existing branch, fully silenced: a
			// missing remote ref prints fatal noise to stderr, and we tolerate
			// fetch failure when the branch already exists locally. We Verify
			// afterwards. The base is not fetched here — only the branch.
			_ = git.Quiet(repoAbs, "fetch", "origin", branchFlag)
			switch {
			case git.Verify(repoAbs, "refs/heads/"+branch):
				render.Info("Creating worktree: %s → %s (existing branch, base: %s)", repo, branch, base)
				gitOrExit(git.Run(repoAbs, "worktree", "add", wt, branch))
			case git.Verify(repoAbs, "refs/remotes/origin/"+branch):
				render.Info("Creating worktree: %s → %s (tracking origin/%s, base: %s)", repo, branch, branch, base)
				gitOrExit(git.Run(repoAbs, "worktree", "add", wt, "-b", branch, "--track", "origin/"+branch))
			default:
				render.Die("Branch '%s' not found locally or on origin in '%s'.", branch, repo)
			}
		} else {
			// New branch: start from origin/<base> after a fetch so the branch
			// isn't silently behind origin from its first commit.
			start := base
			if stackParent == "" {
				start = fetchedBase(repoAbs, repo, base)
			}
			render.Info("Creating worktree: %s → %s (base: %s)", repo, branch, start)
			gitOrExit(git.Run(repoAbs, "worktree", "add", "--no-track", wt, "-b", branch, start))
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
