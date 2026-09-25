---
name: becket
description: >-
  Coordinates a feature across multiple git repositories with the becket CLI.
  becket groups per-repo git worktrees (all on one shared feature
  branch) into a "workspace" and manages their lifecycle: create/adopt/teardown
  workspaces, show cross-repo branch status, add repos, sync/rebase, stack
  features and restack, install each repo's dependencies, garbage-collect
  merged/idle workspaces, and push branches or open PRs. Use this when the
  working directory is managed by becket (a .becket/settings.json platform
  config or a .becket.json workspace manifest is present), when the becket
  command is on PATH, or when the user asks to create, inspect, sync, stack,
  install dependencies for, clean up, or tear down cross-repo feature
  workspaces or worktrees.
---

# becket

becket turns a feature that spans several repos into one **workspace**: a git
worktree per repo, all on a shared feature branch, managed as a unit. Prefer
becket commands over raw `git worktree` for anything in a becket project.

## Detecting a becket project

You're in one if any ancestor directory contains `.becket/settings.json` (the
"platform directory", where the repo clones live) or `.becket.json` (a workspace
manifest). `becket list` shows all workspaces; `becket status` shows the current
one. Each workspace lives at `<platform>/workspaces/<id>/` and holds one
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
- `becket create <id> [--desc TEXT] [--repos a,b] [--base BRANCH] [--branch BRANCH] [--stacked-on PARENT] [--setup]`
  — new workspace + a worktree per repo on a fresh feature branch. Pass
  `--branch <name>` to instead create the workspace on an EXISTING branch
  (e.g. one that lives on origin but hasn't been pulled): becket fetches it
  and creates a tracking worktree without touching the main clone's working
  state. `--branch` is mutually exclusive with `--desc` and `--stacked-on`.
- `becket adopt <id> [--repos a,b]` — wrap repos' already-checked-out branches
  into a new workspace (auto-stashes dirty changes).
- `becket add [id] <repo>` — add another repo to an existing workspace.
- `becket desc [id] <text>` / `becket status set <text>` / `becket status clear`
  — set the description / set or clear a status note.
- `becket teardown [id] [--delete-branches]` — remove the worktrees (and
  optionally the branches) and the workspace dir.
- `becket gc [--apply] [--idle-days N] [--deps-idle-days M]` — remove merged,
  empty (no-work), and idle disposable workspaces, and prune dependency
  directories from idle ones it keeps. Dry-run by default — pass `--apply` to
  act. Never removes the current workspace, a workspace with a stack child,
  or one with uncommitted or unpushed work.

**Sync / ship**
- `becket sync [id]` — rebase every repo onto its base branch.
- `becket restack [id]` — for a stacked workspace, rebase onto the parent's
  current branch tips (use this instead of `sync` when stacked).
- `becket push [id]` — push each repo's branch to origin.
- `becket pr [id]` — open a GitHub PR per repo (needs the `gh` CLI).
- `becket log [id]` — commits per repo since branching.

**Develop**
- `becket deps [id] [--repo NAME] [--force]` — install a repo's dependencies
  with its canonical command; a no-op when the lockfiles haven't changed, so
  run it freely instead of the package manager directly. `create` never
  installs dependencies unless `--setup` is passed — run this yourself first.
- `becket setup [id]` — run each repo's configured setup commands (also runs
  `deps` first for repos that configure it).
- `becket dev [id] [--repo NAME]` — run repos' `dev` commands in the foreground:
  all at once with per-repo-prefixed output (Ctrl-C stops all), or just one with
  `--repo`. Starts docker services first if configured.
- `becket shell [id]` — print a workspace path; with `eval "$(becket shell-init)"`
  in the shell rc, `becket shell <id>` also `cd`s into it.

## Stacking

`becket create child --stacked-on parent` bases the child's branches on the
parent's branches (a feature on top of a feature). After the parent advances,
run `becket restack child` to rebase the child onto the parent's new tips.
`becket sync` refuses stacked workspaces and points you to `restack`.

## Typical workflow

```bash
becket create proj-42 --desc "dark mode" --repos web,api    # create workspace
becket shell proj-42                                        # cd in (with shell-init)
becket deps                                                 # install deps for the repo you're in
# … edit + commit in each repo's worktree …
becket status proj-42                                       # check across repos
becket sync proj-42                                         # rebase onto base
becket push proj-42 && becket pr proj-42                    # ship
becket teardown proj-42 --delete-branches                   # clean up
```

`create` never installs dependencies (`--setup` also runs setup commands, but
is not the default) — run `becket deps` yourself once you're ready to run a
repo's code.

### Resume an existing remote branch

When the branch already lives on origin (you haven't pulled it), use `--branch`
to spin up a workspace on it directly — becket fetches it and creates a
tracking worktree without touching the main clone's working state:

```bash
becket create mul-2418 --branch feature/mul-2418-thing --repos ml-scribe
becket shell mul-2418
# … continue work …
```

## Failure modes to pre-empt

Environment drift inside a workspace is almost always fixed by reinstalling
dependencies: `becket deps --force` (run inside the affected repo).

- **Branch starts behind origin** — becket branches new worktrees from
  `origin/<base>` (fetched at create), but a workspace created by an older
  becket, offline, or from a repo without a remote starts from the local base,
  which may be stale. If `becket status` shows the branch behind, or the code
  predates recent merges, run `becket sync` (rebases onto `origin/<base>`).
- **Optional dependencies vanish** — `deps.run` is the canonical way to build
  each repo's environment. Package-manager commands run directly can silently
  undo them (e.g. a bare `uv run`/`uv sync` re-syncs the venv without the
  extras `deps.run` installs). If imports that worked stop resolving, re-run
  `becket deps --force` instead of debugging the interpreter.
- **`bad interpreter` / shebangs pointing at a dead path** — virtualenvs embed
  absolute paths, so a venv created before a workspace was moved (e.g. the
  pre-1.x `.becket/workspaces/` → `workspaces/` migration) breaks afterwards.
  Recreate it with `becket deps --force`.
- **Imports resolve to a deleted package after rebase** — a package removed
  upstream can survive locally as an orphaned `__pycache__` dir that shadows
  imports. `becket sync`/`restack` delete pure-cache orphans automatically; if
  imports still misresolve after a rebase, inspect `git status --ignored`.
