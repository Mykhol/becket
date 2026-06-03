# shellcheck shell=bash
# `becket adopt` — wrap an already-existing branch into a workspace (vs. create,
# which makes a fresh branch). Detects the repo's current branch; refuses if the
# repo is sitting on its base branch.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service

note "adopt refuses a repo that is on its base branch (nothing to adopt)"
run_becket adopt nope --repos service

note "adopt the repo's current feature branch into a new workspace"
# Simulate pre-existing work: a feature branch with a commit, left checked out.
_git -C "$PLATFORM/service" checkout -b feature/existing-work
printf 'pre-existing\n' >"$PLATFORM/service/EXISTING.md"
_git -C "$PLATFORM/service" add EXISTING.md
_git -C "$PLATFORM/service" commit -m "wip: pre-existing work"

run_becket adopt rescued --repos service
show_manifest rescued
show_branches service
show_worktrees service
