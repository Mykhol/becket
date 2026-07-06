package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/workspace"
)

// writeAgentsMD regenerates a workspace's AGENTS.md from its manifest and the
// platform config, porting write_agents_md. (Content is not golden-checked, but
// kept faithful for real-world use.)
func writeAgentsMD(wsPath string, m *workspace.Manifest, settings config.Settings) {
	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }

	branches := make([]string, 0, len(m.Order))
	seen := map[string]bool{}
	for _, name := range m.Order {
		branches = append(branches, m.Repos[name].Branch)
		seen[m.Repos[name].Branch] = true
	}
	allSame := len(seen) == 1 && len(branches) > 0

	w("# Workspace: " + m.ID)
	w("")
	if m.Description != "" {
		w(m.Description)
		w("")
	}
	w("This directory is a **becket workspace** — a set of git worktrees grouped under a shared feature branch.")
	w("")
	w("## Key Facts")
	w("")
	if allSame {
		w("- **Branch**: `" + branches[0] + "` (same branch name across all repos)")
	}
	w("- **Workspace ID**: `" + m.ID + "`")
	if m.Status != nil && m.Status.Text != "" {
		line := "- **Status**: " + m.Status.Text
		if m.Status.UpdatedBy != "" {
			line += " *(set by " + m.Status.UpdatedBy
			if m.Status.UpdatedAt != "" {
				line += " at " + m.Status.UpdatedAt
			}
			line += ")*"
		}
		w(line)
	}
	w("")
	w("## Repos")
	w("")
	if allSame {
		w("| Directory | Base Branch |")
		w("|-----------|-------------|")
		for _, name := range m.Order {
			w("| `" + name + "/` | `" + m.Repos[name].Base + "` |")
		}
	} else {
		w("| Directory | Branch | Base Branch |")
		w("|-----------|--------|-------------|")
		for _, name := range m.Order {
			w("| `" + name + "/` | `" + m.Repos[name].Branch + "` | `" + m.Repos[name].Base + "` |")
		}
	}
	w("")

	hasCommands := false
	for _, name := range m.Order {
		rc := settings.Repos[name]
		if len(rc.Setup) > 0 || rc.Dev != "" {
			hasCommands = true
			break
		}
	}
	if hasCommands {
		w("## Dev Commands")
		w("")
		for _, name := range m.Order {
			rc := settings.Repos[name]
			if len(rc.Setup) == 0 && rc.Dev == "" {
				continue
			}
			w("### " + name)
			w("")
			if len(rc.Setup) > 0 {
				w("**Setup:**")
				w("```bash")
				for _, c := range rc.Setup {
					w(c)
				}
				w("```")
				w("")
			}
			if rc.Dev != "" {
				w("**Dev server:** `" + rc.Dev + "`")
				w("")
			}
		}
	}

	w("## Documentation")
	w("")
	w("The `docs/` directory is for all documentation related to this feature — specs, designs, notes, research, etc. Keep feature docs here rather than inside individual repos.")
	w("")
	w("## Important")
	w("")
	w("- Each subdirectory is a **git worktree**, not a clone. Commits here are real and affect the main repository.")
	if allSame {
		w("- All repos share the same feature branch name, so cross-repo changes stay in sync.")
	} else {
		w("- Repos in this workspace may be on different feature branches (adopted from existing work).")
	}
	w("- Do not run `git worktree` commands directly. Use the `becket` CLI to manage this workspace.")
	w("- To check status: `becket status " + m.ID + "`")
	w("")
	w("## Troubleshooting")
	w("")
	w("Environment drift is almost always fixed by re-running the repos' setup commands: `becket setup " + m.ID + "`.")
	w("")
	w("- **Imports or optional dependencies missing** after running package-manager commands directly — e.g. a bare `uv run`/`uv sync` re-syncs the venv *without* the extras the setup commands install. Re-run `becket setup " + m.ID + "`.")
	w("- **`bad interpreter` errors, or tool shebangs pointing at a path that no longer exists** — the virtualenv predates a workspace move (venvs embed absolute paths). Re-run `becket setup " + m.ID + "` to recreate it.")
	w("- **Imports resolving to a deleted package after `becket sync`/`restack`** — a package deleted upstream can survive locally as an orphaned `__pycache__` that shadows imports. becket removes pure-cache orphans after each rebase; if imports still misresolve, check `git status --ignored`.")
	w("- **Branch starts behind origin** — the workspace was created from a stale local base by an older becket. `becket sync " + m.ID + "` rebases every repo onto `origin/<base>`.")
	w("")

	_ = os.WriteFile(filepath.Join(wsPath, "AGENTS.md"), []byte(b.String()), 0o644)
}
