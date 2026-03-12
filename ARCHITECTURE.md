# Architecture

## Current Architecture

Becket is a single bash script (`bin/becket`, ~585 lines). It uses `python3` for JSON manipulation (reading/writing settings and manifests). There are no external dependencies beyond:

- **Bash 4+**
- **Git 2.15+** (worktree support)
- **Python 3** (available by default on macOS and most Linux)

Installation is a single file copy via `make install` to `~/.local/bin/becket`.

## Key Concepts

- **Platform directory**: The parent directory containing your repo clones. Not a becket-specific concept — it's just wherever your repos live (e.g., `~/Developer/MyProject/`). Becket discovers it by walking up from CWD looking for `.becket/settings.json`.

- **Workspace**: A directory grouping git worktrees from multiple repos under a single feature identifier. Each workspace contains:
  - Git worktrees (one per selected repo, all on the same feature branch)
  - A `docs/` folder for shared feature documentation
  - A `.becket.json` manifest tracking workspace state
  - An `AGENTS.md` file with generated context for AI agents

- **Settings**: `.becket/settings.json` at the platform directory root. Stores the list of known repos (with paths and default base branches) and the branch prefix (default: `feature/`).

- **Manifest**: `.becket.json` per workspace. Tracks the workspace id, description, creation date, and per-repo branch/base information.

## File Layout

```
<platform-dir>/
├── .becket/
│   ├── settings.json          # platform config (repos, branch prefix)
│   └── workspaces/
│       └── <workspace-id>/
│           ├── .becket.json   # workspace manifest
│           ├── AGENTS.md      # generated context for AI agents
│           ├── docs/          # shared feature docs
│           ├── <repo-1>/      # git worktree
│           └── <repo-2>/      # git worktree
├── repo-1/                    # main clone
└── repo-2/                    # main clone
```

## Commands

| Command | What it does |
|---------|-------------|
| `init` | Scans CWD for git repos, creates `.becket/settings.json` |
| `create <id>` | Creates a workspace directory, adds git worktrees for selected repos, writes manifest and `AGENTS.md` |
| `list` | Lists all workspaces from `.becket/workspaces/*/` |
| `status [id]` | Shows branch, clean/dirty status, and ahead/behind counts per repo |
| `teardown <id>` | Removes worktrees via `git worktree remove`, deletes workspace directory. Optionally deletes branches. |
| `add <id> <repo>` | Adds a new repo worktree to an existing workspace, reusing the workspace's branch name |

## Design Constraints

- **Language rewrite is planned** — Bash is the prototype. A rewrite to a compiled or scripted language (TBD) is on the roadmap. Current code should be treated as functional but not final.
- **Must stay portable** — No heavy runtime dependencies. The tool should work on macOS and Linux with standard tooling.
- **Agent context is a first-class output** — `AGENTS.md` generation is core functionality, not a nice-to-have. Any new feature that changes workspace structure should update agent context accordingly.
