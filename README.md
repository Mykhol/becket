# becket

> Work on one feature across many repositories with a single command.

[![CI](https://github.com/Mykhol/becket/actions/workflows/ci.yml/badge.svg)](https://github.com/Mykhol/becket/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Mykhol/becket)](https://github.com/Mykhol/becket/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

When a feature spans several repositories — a frontend, a backend, a shared
library — you end up juggling matching branches across a pile of clones. **becket
collapses that into a single _workspace_:** coordinated git worktrees, one per
repo, all on a shared feature branch, managed as a unit.

```console
$ becket create proj-42 --desc "dark mode" --repos web,api
▸ Created workspace: ~/Developer/Platform/.becket/workspaces/proj-42
▸ Creating worktree: web → feature/proj-42-dark-mode (base: main)
▸ Creating worktree: api → feature/proj-42-dark-mode (base: main)
▸ Workspace proj-42 ready

$ becket status proj-42

Workspace: proj-42 — dark mode

  REPO   BRANCH                    STATUS   AHEAD/BEHIND   LAST COMMIT
  ─────  ────────────────────────  ───────  ────────────   ───────────
  web    feature/proj-42-dark-mode  clean    +3 / -0        2h ago
  api    feature/proj-42-dark-mode  clean    +1 / -0        4h ago
```

## Features

- **One workspace, many repos** — create coordinated worktrees across your repos on a shared feature branch in one command.
- **Cross-repo status at a glance** — branch, clean/dirty, ahead/behind, and last commit for every repo together.
- **Stacked features** — build a feature on top of another's branches (`--stacked-on`) and `restack` onto the parent's latest tips.
- **Adopt existing work** — wrap branches you've already started into a workspace without losing changes.
- **Sync & ship** — `sync`, `push`, `log`, and `pr` (GitHub) operate across all repos at once.
- **Single static binary** — no runtime dependencies beyond `git`.

## Install

**Prebuilt binary** — download the archive for your platform from the
[latest release](https://github.com/Mykhol/becket/releases/latest), extract, and
put `becket` on your `PATH` (macOS & Linux, amd64 & arm64).

**Go toolchain**

```bash
go install github.com/Mykhol/becket@latest
```

**From source**

```bash
git clone https://github.com/Mykhol/becket && cd becket && make install
```

Requires **Git 2.15+** (`becket dev` also needs `tmux`; `becket pr` needs the
[`gh`](https://cli.github.com) CLI).

## Usage

Run `becket init` once in the directory that holds your repo clones (your
"platform directory"), then work in feature workspaces:

```bash
becket init                                   # discover repos, write .becket/settings.json
becket create proj-42 --desc "dark mode"      # workspace + a worktree per repo
becket status proj-42                          # branch status across all repos
becket add proj-42 api                         # bring another repo into the workspace
becket sync proj-42                            # rebase every repo onto its base branch
becket push proj-42 && becket pr proj-42       # push branches, open PRs
becket teardown proj-42 --delete-branches      # remove worktrees + branches
```

Add shell integration to your `~/.zshrc` / `~/.bashrc` so `becket shell <id>`
changes directory and tab-completion works:

```bash
eval "$(becket shell-init)"
```

## Concepts

A **workspace** lives at `.becket/workspaces/<id>/` and contains a git worktree
per selected repo (all on one feature branch), a shared `docs/` folder, a
`.becket.json` manifest, and a generated `AGENTS.md` for AI agents. Workspaces
can be **stacked** so one feature builds on another's branches; `becket restack`
rebases a stack onto its parent's current tips.

## Configuration

`becket init` writes `.becket/settings.json` at your platform directory root:

```json
{
  "$schema": "./settings.schema.json",
  "repos": {
    "web": { "path": "./web", "defaultBase": "main" },
    "api": { "path": "./api", "defaultBase": "main" }
  },
  "branchPrefix": "feature/"
}
```

Optional keys add per-repo `setup`/`dev`/`env` commands (for `becket setup` and
`becket dev`), top-level `files` to copy into each workspace, and `docker` /
`session` for the dev environment. `becket upgrade` migrates older configs.

## Command reference

| Command | Description |
|---|---|
| `init` | Scan the current directory for repos and write `.becket/settings.json` |
| `create <id> [flags]` | Create a workspace and a worktree per repo (`--desc`, `--repos`, `--base`, `--stacked-on`, `--setup`) |
| `adopt <id> [flags]` | Wrap each repo's existing branch into a new workspace |
| `add [id] <repo>` | Add a repo to an existing workspace |
| `list [--output json]` | List active workspaces |
| `status [id] [--output json]` | Show branch status across repos |
| `status set <text>` / `status clear` | Set or clear a workspace status note |
| `desc [id] <text>` | Set a workspace description |
| `sync [id]` | Rebase every repo onto its base branch |
| `restack [id]` | Rebase a stacked workspace onto its parent's tips |
| `push [id]` / `pr [id]` / `log [id]` | Push branches / open GitHub PRs / show commits |
| `setup [id]` / `dev [id] [--detach]` | Run setup commands / start the dev environment |
| `shell [id]` / `shell-init` | Print a workspace path / emit shell integration |
| `teardown [id] [--delete-branches]` | Remove a workspace's worktrees (and branches) |
| `upgrade` / `stats` | Migrate config & schemas / show local usage stats |

Run `becket <command> --help` for full flag details.

## Development

```bash
make build      # build ./becket-go
make test       # go test ./...  (unit tests + testscript E2E suite)
make install    # build and install to ~/.local/bin/becket
```

See [`tests/README.md`](tests/README.md) for the test layout.

## License

[MIT](LICENSE)
