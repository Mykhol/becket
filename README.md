# becket

Cross-repo worktree CLI. Create feature-scoped workspaces containing coordinated git worktrees across multiple repositories.

## Quick Start

```bash
# Install
make install   # copies to ~/.local/bin/becket

# Initialize in your platform directory (where your repos live)
cd ~/Developer/Example/Platform
becket init

# Create a feature workspace
becket create proj-42 --desc "dark mode"

# Check status
becket status proj-42

# Add another repo later
becket add proj-42 Service-Admin

# List all workspaces
becket list

# Clean up
becket teardown proj-42 --delete-branches
```

## How It Works

Becket manages **workspaces** — directories that group git worktrees from multiple repos under a single feature identifier. Each workspace gets:

- A worktree per selected repo, all on the same feature branch
- A shared `docs/` folder for API contracts, notes, etc.
- A `.becket.json` manifest tracking the workspace state

## Config

Run `becket init` in a directory containing your repo clones. It creates a `.becket.json`:

```json
{
  "workspacesDir": "~/workspaces",
  "repos": {
    "ml-service": { "path": "./ml-service", "defaultBase": "main" },
    "service-fe-v2": { "path": "./service-fe-v2", "defaultBase": "main" }
  },
  "branchPrefix": "feature/"
}
```

## Commands

| Command | Description |
|---------|-------------|
| `becket init` | Create `.becket.json` in current directory |
| `becket create <id> [--desc TEXT] [--repos r1,r2] [--base BRANCH]` | Create feature workspace |
| `becket list` | List all active workspaces |
| `becket status [id]` | Show branch status across repos |
| `becket teardown <id> [--delete-branches]` | Remove worktrees + workspace |
| `becket add <id> <repo>` | Add a repo to existing workspace |

## Requirements

- Bash 4+
- Git 2.15+ (worktree support)
- Python 3 (for JSON handling; available by default on macOS and most Linux)

## License

MIT
