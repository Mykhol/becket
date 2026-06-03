# shellcheck shell=bash
# `becket push` (push feature branches to origin) and `becket log` (commits on
# the feature branch since base). Both run fully offline against the bare origin.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service
run_becket create ws --repos service >/dev/null 2>&1

# Put one commit on the feature branch so log has something to show.
WT="$PLATFORM/.becket/workspaces/ws/service"
printf 'feature work\n' >"$WT/FEATURE.md"
_git -C "$WT" add FEATURE.md
_git -C "$WT" commit -m "feat: add FEATURE.md"

note "log shows commits on the feature branch ahead of base"
run_becket log ws

note "push the feature branch to origin"
run_becket push ws
show_branches service

note "log still works after the branch exists on origin"
run_becket log ws
