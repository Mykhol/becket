# Product Vision

## Problem

Cross-repo feature work is painful. When a feature spans multiple repositories, developers must manually create worktrees or branches across N repos, keep branch names in sync, and track the state of each independently. There's no unified view of a feature's progress across repos.

This problem is worse for AI coding agents. Agents have no structured way to get multi-repo context for a feature — they can only see the single repo they're dropped into, with no awareness of related changes happening in sibling repos.

## Vision

Becket is the **workspace layer for multi-repo development**.

Today, it manages coordinated git worktrees — creating, syncing, and tearing down feature-scoped workspaces that span multiple repositories. Each workspace groups worktrees under a shared branch name and provides structured context (via `AGENTS.md` and a shared `docs/` folder) so that both humans and AI agents can understand the full scope of a feature.

The end state is an **orchestration platform** where autonomous agents — developers, PMs, testers — each get full feature context and can collaborate within a workspace. The roadmap includes:

- Desktop application and/or TUI for managing workspaces and watching agent progress
- Agent orchestration: spawn and coordinate multiple AI agents per workspace
- Rich context injection: ticket descriptions, design docs, API contracts available to agents by default

## Target User

Any developer (or AI agent) working across multiple related git repositories. Becket is open source and general purpose — not tied to any specific language, framework, or project structure.

## Key Principles

- **Agent-first**: Every feature should consider how it serves AI agents, not just humans. Agent context generation (`AGENTS.md`) is a first-class output, not an afterthought.
- **Git-native**: Build on git primitives (worktrees, branches) rather than inventing abstractions. Becket doesn't replace git — it coordinates it.
- **Convention over configuration**: Sensible defaults, minimal setup, just works. A developer should go from `becket init` to a working multi-repo workspace in seconds.

## Non-Goals

- **Not a CI/CD system** — Becket orchestrates local workspaces, not builds or deploys.
- **Not repo-opinionated** — Works with any git repositories regardless of language, framework, or project structure.
