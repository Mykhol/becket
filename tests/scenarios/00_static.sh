# shellcheck shell=bash
# Static output and global error paths — no platform required.
source "$(dirname "$0")/../lib/harness.sh"

# Pure-output commands that load no config. Run from a config-less dir so the
# upward walk finds nothing (proves they don't need a platform).
cd "$SANDBOX"

note "version"
run_becket version
run_becket --version
run_becket -v

note "help (explicit, and as the default command)"
run_becket help
run_becket

note "shell-init (shell wrapper + completion bootstrap)"
run_becket shell-init

note "unknown command is rejected"
run_becket frobnicate

note "commands that require a platform die with guidance when none exists"
run_becket list
run_becket status
