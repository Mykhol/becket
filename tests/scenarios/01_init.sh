# shellcheck shell=bash
# `becket init` — platform bootstrap and repo discovery.
source "$(dirname "$0")/../lib/harness.sh"

note "init in a directory with no git repos"
mk_platform
run_becket init
show_settings

note "init discovers sibling git repos and records their default base"
# Fresh platform with two repos (both default to main via the fixture).
rm -rf "$PLATFORM"
mk_platform
mk_repo service
mk_repo service-admin
run_becket init
show_settings

note "init refuses to clobber an existing config"
run_becket init
