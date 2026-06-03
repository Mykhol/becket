# shellcheck shell=bash
# `becket sync` when the rebase onto origin/<base> hits a conflict.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service
run_becket create ws --repos service >/dev/null 2>&1

# Feature branch and base branch edit the same line → guaranteed rebase conflict.
commit_file "$(wt ws service)" README.md "FEATURE VERSION" "feat: rewrite readme"
_git -C "$PLATFORM/service" checkout main
commit_file "$PLATFORM/service" README.md "MAIN VERSION" "main: rewrite readme"
_git -C "$PLATFORM/service" push origin main

note "sync reports the rebase conflict"
run_becket sync ws
