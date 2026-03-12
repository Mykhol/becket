# Roadmap

## Current (v0.1)

Bash CLI with core workspace lifecycle management:

- `init` — discover repos and create platform config
- `create` — create feature workspaces with coordinated git worktrees
- `list` — list all active workspaces
- `status` — show branch status across repos (clean/dirty, ahead/behind)
- `teardown` — remove worktrees and clean up workspace directories
- `add` — add a repo to an existing workspace
- `AGENTS.md` generation per workspace with structured context for AI agents

## Near-term

- **Language rewrite** — Rewrite from bash to a compiled or scripted language (TBD). Bash is the prototype; the rewrite will enable richer features, better error handling, and easier contribution.
- **Cross-repo PR creation/sync** — Create and manage pull requests across all repos in a workspace from a single command.
- **Richer agent context** — Full feature context by default: ticket descriptions, design docs, API contracts. Agents should be able to access everything they need to understand and implement a feature without leaving the workspace.
- **Improved AGENTS.md / CLAUDE.md generation** — More detailed, more useful generated context files tailored to different AI agent platforms.

## Long-term Vision

- **Agent orchestration platform** — Spawn and coordinate multiple AI agents per workspace, each with a distinct role (developer, PM, tester). Agents get full product context and can collaborate: discussing tradeoffs, reviewing each other's work, and driving features to completion.
- **Desktop application and/or TUI** — Visual interface for managing workspaces, watching agent progress, and interacting with running agents.
- **Rich collaboration** — Agents can communicate within a workspace, share findings, flag blockers, and hand off work — moving from isolated code generation toward genuine multi-agent collaboration on features.
