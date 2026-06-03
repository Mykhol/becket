# shellcheck shell=bash
# Untested flags/branches on already-covered commands:
#   create --base, teardown (keep branches), add (id detected from CWD).
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service service-admin
_git -C "$PLATFORM/service" branch develop   # an alternate base branch to target

note "create --base overrides the base branch the worktree is cut from"
run_becket create feat --repos service --base develop
show_manifest feat

note "teardown without --delete-branches removes the worktree but keeps the branch"
run_becket teardown feat
show_branches service
show_tree

note "add with the workspace id detected from the current directory (single-arg form)"
run_becket create multi --repos service >/dev/null 2>&1
cd "$(wt multi service)"
run_becket add service-admin
show_manifest multi
