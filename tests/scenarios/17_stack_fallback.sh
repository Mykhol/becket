# shellcheck shell=bash
# Stacking a child onto a parent that doesn't include one of the child's repos:
# that repo falls back to its defaultBase (with a warning) instead of a parent
# branch.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform alpha beta

note "parent workspace covers only one of the two repos"
run_becket create parent --repos alpha >/dev/null 2>&1
run_becket push parent >/dev/null 2>&1

note "child stacks on parent across both repos: beta falls back to its base"
run_becket create child --stacked-on parent --repos alpha,beta
show_manifest child
