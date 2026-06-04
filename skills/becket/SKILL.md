---
name: becket
description: >-
  Coordinates a feature across multiple git repositories with the becket CLI.
  becket groups per-repo git worktrees (all on one shared feature
  branch) into a "workspace" and manages their lifecycle: create/adopt/teardown
  workspaces, show cross-repo branch status, add repos, sync/rebase, stack
  features and restack, and push branches or open PRs. Use this when the working
  directory is managed by becket (a .becket/settings.json platform config or a
  .becket.json workspace manifest is present), when the becket command is on
  PATH, or when the user asks to create, inspect, sync, stack, or tear down
  cross-repo feature workspaces or worktrees.
---

# becket

becket turns a feature that spans several repos into one **workspace**: a git
worktree per repo, all on a shared feature branch, managed as a unit. Prefer
becket commands over raw `git worktree` for anything in a becket project.

## Detecting a becket project

You're in one if any ancestor directory contains `.becket/settings.json` (the
"platform directory", where the repo clones live) or `.becket.json` (a workspace
manifest). `becket list` shows all workspaces; `becket status` shows the current
one. Each workspace lives at `<platform>/.becket/workspaces/<id>/` and holds one
worktree subdirectory per repo plus a generated `AGENTS.md` describing it — read
that `AGENTS.md` when you start work inside a workspace.

## Conventions (important)

- **Each subdirectory under a workspace is a git worktree, not a clone.** Commits
  there are real and affect the underlying repository.
- **Don't run `git worktree add/remove` yourself** — use `becket create`/`add`/
  `teardown` so the manifest stays in sync.
- All repos in a workspace normally share the **same feature branch name**.
- Run `becket <command> --help` for exact flags. Commands take an optional `[id]`
  that is auto-detected from the current directory when omitted.

## Commands

**Set up / inspect**
- `becket init` — in the platform directory, scan for repos and write config.
- `becket list [--output json]` — list workspaces.
- `becket status [id] [--output json]` — branch, clean/dirty, ahead/behind, and
  last-commit per repo. Use this first to understand a workspace.

**Create / modify**
- `becket create <id> [--desc TEXT] [--repos a,b] [--base BRANCH] [--stacked-on PARENT] [--setup]`
  — new workspace + a worktree per repo on a fresh feature branch.
- `becket adopt <id> [--repos a,b]` — wrap repos' already-checked-out branches
  into a new workspace (auto-stashes dirty changes).
- `becket add [id] <repo>` — add another repo to an existing workspace.
- `becket desc [id] <text>` / `becket status set <text>` / `becket status clear`
  — set the description / set or clear a status note.
- `becket teardown [id] [--delete-branches]` — remove the worktrees (and
  optionally the branches) and the workspace dir.

**Sync / ship**
- `becket sync [id]` — rebase every repo onto its base branch.
- `becket restack [id]` — for a stacked workspace, rebase onto the parent's
  current branch tips (use this instead of `sync` when stacked).
- `becket push [id]` — push each repo's branch to origin.
- `becket pr [id]` — open a GitHub PR per repo (needs the `gh` CLI).
- `becket log [id]` — commits per repo since branching.

**Develop**
- `becket setup [id]` — run each repo's configured setup commands.
- `becket dev [id] [--detach]` — start the dev environment (docker + tmux).
- `becket shell [id]` — print a workspace path; with `eval "$(becket shell-init)"`
  in the shell rc, `becket shell <id>` also `cd`s into it.

## Stacking

`becket create child --stacked-on parent` bases the child's branches on the
parent's branches (a feature on top of a feature). After the parent advances,
run `becket restack child` to rebase the child onto the parent's new tips.
`becket sync` refuses stacked workspaces and points you to `restack`.

## Typical workflow

```bash
becket create proj-42 --desc "dark mode" --repos web,api   # create
becket shell proj-42                                        # cd in (with shell-init)
# … edit + commit in each repo's worktree …
becket status proj-42                                       # check across repos
becket sync proj-42                                         # rebase onto base
becket push proj-42 && becket pr proj-42                    # ship
becket teardown proj-42 --delete-branches                   # clean up
```
