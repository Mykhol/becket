# shellcheck shell=bash
# Stacked workspaces: --stacked-on, stack-aware list/status, and restack.
# Restack fetches/rebases onto origin/<parent-branch>, so the parent must be
# pushed to its (local, bare) origin first.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service

note "create the base workspace and push it so its branch exists on origin"
run_becket create base-feature --desc "base" --repos service
run_becket push base-feature

note "create a workspace stacked on top of it"
run_becket create stacked-feature --stacked-on base-feature --repos service
show_manifest base-feature
show_manifest stacked-feature
show_branches service

note "--stacked-on cannot be combined with --base"
run_becket create bad-stack --stacked-on base-feature --base main --repos service

note "list and status reflect the stack relationship"
run_becket list
run_becket status stacked-feature

note "restack the child onto the parent's branch"
run_becket restack stacked-feature
show_manifest stacked-feature

note "restack refuses a workspace that isn't stacked"
run_becket restack base-feature
