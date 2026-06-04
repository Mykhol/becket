package cli

import (
	"fmt"
	"strings"

	"github.com/Mykhol/becket/internal/render"
)

// helpText mirrors cmd_help. {B}/{R} mark the bold section headers, expanded on a
// TTY and stripped otherwise (matching the bash [[ -t 1 ]] color gate). %s is the
// version.
const helpText = `{B}becket{R} v%s — Cross-repo worktree CLI

{B}USAGE{R}
  becket <command> [options]

{B}PLATFORM{R}
  init                          Initialize .becket/ in current directory
  list [--output json]           List all active workspaces
  stats                         Show local command usage statistics
  upgrade                       Update config + schemas to latest version

{B}WORKSPACE{R}
  create <id> [options]         Create workspace + worktrees
  adopt <id> [options]          Adopt existing branches into a new workspace
  teardown [id] [options]       Remove worktrees + workspace
  add [id] <repo>               Add a repo to an existing workspace

{B}WORKSPACE — DEVELOP{R}
  shell [id]                    Print workspace directory path (use with shell-init)
  shell-init                    Output shell wrapper for eval (enables 'becket shell' as cd)
  dev [id] [--detach]           Start dev environment (docker + tmux)
                                --detach: don't attach (auto when no TTY, e.g. agents)
  setup [id]                    Run setup commands for workspace repos

{B}WORKSPACE — SYNC & SHIP{R}
  status [id] [--output json]   Show branch status across repos
  status set <text> [--by who]  Set workspace status message
  status clear [id]             Clear workspace status message
  desc [id] <text>              Set workspace description
  log [id]                      Show commits across all repos since branching
  sync [id]                     Rebase all repos against their base branch
  restack [id]                  Rebase a stacked workspace onto its parent's current tips
  push [id]                     Push all repo branches to origin
  pr [id]                       Open GitHub PRs for all repos (requires gh)

{B}CREATE OPTIONS{R}
  --desc TEXT                   Description (appended to branch name)
  --repos r1,r2                 Repos to include (default: interactive)
  --base BRANCH                 Override base branch for all repos
  --stacked-on PARENT_ID        Stack this workspace on another (parent's branches become the base)
  --setup                       Run setup commands after creating workspace

{B}ADOPT OPTIONS{R}
  --repos r1,r2                 Repos to include (default: interactive)
  --base BRANCH                 Override base branch for all repos
  --setup                       Run setup commands after adopting

{B}TEARDOWN OPTIONS{R}
  --delete-branches             Also delete feature branches

{B}SHELL INTEGRATION{R}
  Add to your .zshrc or .bashrc:    eval "$(becket shell-init)"

{B}ENVIRONMENT{R}
  BECKET_CONFIG                 Path to settings.json (overrides discovery)

{B}CONFIG (settings.json){R}
  repos                         Map of repo names to path + defaultBase
    setup                       List of commands to run during first-time setup
    dev                         Command to start the dev server (for tmux)
    env                         Environment variables for setup and dev
  branchPrefix                  Prefix for feature branches (default: feature/)
  files                         List of files to copy into each workspace root
  docker                        Path to docker-compose file (relative to workspace)
  session                       Tmux session name for 'becket dev' (default: workspace ID)

{B}FILES{R}
  .becket/settings.json         Platform config
  .becket/workspaces/<id>/      Workspace directories
`

func printHelp() {
	b, r := "", ""
	if render.IsTTY() {
		b, r = "\033[1m", "\033[0m"
	}
	out := strings.NewReplacer("{B}", b, "{R}", r).Replace(helpText)
	fmt.Printf(out, appVersion)
}
