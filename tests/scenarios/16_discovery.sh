# shellcheck shell=bash
# `becket init` default-base detection: a repo whose only branch is `master`
# should be recorded with defaultBase "master", `main` repos with "main".
source "$(dirname "$0")/../lib/harness.sh"

mk_platform
mk_repo modern              # default branch: main
mk_repo_branch legacy master

note "init records each repo's actual default base (main vs master)"
run_becket init
show_settings
