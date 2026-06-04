# becket

Cross-repo git-worktree workspaces. Create feature-scoped workspaces that group
coordinated git worktrees — one per repo, all on the same feature branch — across
multiple repositories.

## Install

```bash
# Go toolchain
go install github.com/Mykhol/becket@latest

# From source (builds + installs to ~/.local/bin/becket)
git clone https://github.com/Mykhol/becket && cd becket && make install
```

Prebuilt binaries (macOS/Linux, amd64/arm64) are attached to each
[release](https://github.com/Mykhol/becket/releases/latest).

becket is a single static binary; the only runtime requirement is **Git 2.15+**.
`becket dev` additionally needs `tmux`, and `becket pr` needs the `gh` CLI.

## Quick start

```bash
# Initialize in your platform directory (where your repo clones live)
cd ~/Developer/Platform
becket init

# Create a feature workspace
becket create proj-42 --desc "dark mode"

# Check status across all repos
becket status proj-42

# Add another repo later
becket add proj-42 api

# List all workspaces
becket list

# Clean up
becket teardown proj-42 --delete-branches
```

To make `becket shell` cd into a workspace, add to your shell rc:

```bash
eval "$(becket shell-init)"
```

## How it works

becket manages **workspaces** — directories that group git worktrees from
multiple repos under one feature identifier. Each workspace gets:

- a worktree per selected repo, all on the same feature branch,
- a shared `docs/` folder for specs, notes, and contracts,
- a `.becket.json` manifest tracking workspace state,
- a generated `AGENTS.md` with context for AI agents.

Workspaces can be **stacked** (`--stacked-on`) so a feature builds on top of
another's branches, with `becket restack` to rebase onto the parent's tips.

## Configuration

`becket init` scans the current directory for git repos and writes
`.becket/settings.json`:

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

Optional keys: per-repo `setup`/`dev`/`env` (for `becket setup`/`dev`),
top-level `files` (copied into each workspace), `docker`, and `session`.

## Commands

| Command | Description |
|---------|-------------|
| `init` | Scan CWD for repos, write `.becket/settings.json` |
| `create <id> [--desc] [--repos] [--base] [--stacked-on] [--setup]` | Create a workspace + worktrees |
| `adopt <id> [--repos] [--base] [--setup]` | Wrap existing branches into a new workspace |
| `add [id] <repo>` | Add a repo to an existing workspace |
| `list [--output json]` | List active workspaces |
| `status [id] [--output json]` / `status set\|clear` | Branch status; set/clear a status note |
| `desc [id] <text>` | Set a workspace description |
| `sync [id]` / `restack [id]` | Rebase onto base / onto a stack parent's tips |
| `push [id]` / `pr [id]` / `log [id]` | Push branches / open PRs (gh) / show commits |
| `setup [id]` / `dev [id] [--detach]` | Run setup commands / start dev env (tmux) |
| `shell [id]` / `shell-init` | Print workspace path / shell integration |
| `teardown [id] [--delete-branches]` | Remove worktrees + workspace |
| `upgrade` / `stats` | Migrate config/schemas / local usage stats |

## Development

```bash
make build      # build ./becket-go
make test       # go test ./... (unit + testscript E2E suite)
make install    # build + install to ~/.local/bin/becket
```

See [`tests/README.md`](tests/README.md) for the test layout (per-package unit
tests + testscript `.txtar` end-to-end scenarios).

## License

MIT
