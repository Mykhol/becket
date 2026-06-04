# shellcheck shell=bash
# Core workspace lifecycle: create → list → status → add → desc → teardown.
# This is the headline path the migration must preserve exactly.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service admin

note "create a workspace scoped to one repo, with a description"
run_becket create proj-42 --desc "dark mode" --repos service
show_manifest proj-42
show_branches service
show_worktrees service

note "list shows the new workspace"
run_becket list
run_becket list --output json

note "status (all workspaces) and status for a single workspace"
run_becket status
run_becket status proj-42
run_becket status proj-42 --output json

note "add a second repo to the workspace (reuses the feature branch)"
run_becket add proj-42 admin
show_manifest proj-42
show_branches admin
show_tree

note "change the description after the fact"
run_becket desc proj-42 "real-time dark mode"
show_manifest proj-42

note "creating with no --repos selects all repos (non-TTY default)"
run_becket create dash-9
show_manifest dash-9
run_becket list

note "teardown removes worktrees and (with --delete-branches) the branches"
run_becket teardown proj-42 --delete-branches
show_branches service
show_worktrees service
show_tree
run_becket list
