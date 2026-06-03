# shellcheck shell=bash
# `becket adopt` against a DIRTY repo: it auto-stashes uncommitted changes and
# restores them into the new worktree.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service

# Pre-existing work on a feature branch, with an uncommitted tracked change.
_git -C "$PLATFORM/service" checkout -b feature/wip
commit_file "$PLATFORM/service" WIP.md "work in progress" "wip: start"
printf 'uncommitted edit\n' >>"$PLATFORM/service/README.md"   # dirty, not committed

note "adopt stashes the dirty change and restores it in the worktree"
run_becket adopt rescued --repos service
show_manifest rescued
show_worktrees service

note "the uncommitted change was restored into the worktree"
echo "# worktree status: rescued/service"
git -C "$(wt rescued service)" status --porcelain
echo
