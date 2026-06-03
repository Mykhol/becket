# shellcheck shell=bash
# Status notes (status set/clear) and descriptions (desc), including the
# detect-from-CWD path and desc's flexible argument ordering.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service
run_becket create ws-a --repos service >/dev/null 2>&1

note "set a status note from inside the workspace (id detected from CWD)"
cd "$PLATFORM/.becket/workspaces/ws-a"
run_becket status set "blocked on review" --by alice
show_manifest ws-a

note "status (no id) from inside the workspace shows that workspace"
run_becket status

note "clear the status note"
run_becket status clear
show_manifest ws-a

note "desc with explicit id (run from the platform root)"
cd "$PLATFORM"
run_becket desc ws-a "first description"
show_manifest ws-a

note "desc with id detected from CWD (single-arg form)"
cd "$PLATFORM/.becket/workspaces/ws-a"
run_becket desc "second description"
show_manifest ws-a
