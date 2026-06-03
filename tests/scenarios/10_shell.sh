# shellcheck shell=bash
# `becket shell` — print a workspace's directory path (the value `shell-init`'s
# wrapper cd's into). By id, detected from CWD, and the not-found error.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service
run_becket create ws --repos service >/dev/null 2>&1

note "shell prints the workspace path (by id)"
run_becket shell ws

note "shell detects the workspace from the current directory"
cd "$(wt ws service)"
run_becket shell

note "shell errors on an unknown workspace"
cd "$PLATFORM"
run_becket shell nonexistent
