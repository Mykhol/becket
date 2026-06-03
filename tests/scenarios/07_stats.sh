# shellcheck shell=bash
# `becket stats` — aggregates the usage log that every invocation appends to.
# Counts are deterministic for a fixed command sequence (init from setup, plus
# the calls below, plus stats counting itself).
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service                       # logs: init
run_becket create ws --repos service >/dev/null 2>&1   # logs: create

cd "$PLATFORM/.becket/workspaces/ws"
run_becket status >/dev/null 2>&1           # logs: status (workspace=ws)
run_becket list   >/dev/null 2>&1           # logs: list

note "stats summarizes command counts, workspaces, and platforms"
run_becket stats
