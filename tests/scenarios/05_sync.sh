# shellcheck shell=bash
# `becket sync` — rebase a workspace's branches onto origin/<base>, and its
# refusal to touch a stacked workspace (which must use restack instead).
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service

note "sync an up-to-date workspace (clean no-op rebase onto origin/main)"
run_becket create ws --repos service >/dev/null 2>&1
run_becket sync ws

note "sync picks up new commits on the base branch"
# Advance origin/main by committing in the platform clone and pushing.
_git -C "$PLATFORM/service" checkout main
printf 'upstream change\n' >"$PLATFORM/service/UPSTREAM.md"
_git -C "$PLATFORM/service" add UPSTREAM.md
_git -C "$PLATFORM/service" commit -m "upstream: add UPSTREAM.md"
_git -C "$PLATFORM/service" push origin main
run_becket sync ws
show_worktrees service

note "sync refuses a stacked workspace and points at restack"
run_becket create child --stacked-on ws --repos service >/dev/null 2>&1
run_becket sync child
