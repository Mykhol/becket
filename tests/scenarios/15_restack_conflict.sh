# shellcheck shell=bash
# `becket restack` when rebasing the child onto the parent's advanced branch
# hits a conflict. The parent only ever fast-forwards (no force-push needed).
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service

# Parent: commit A on the same line, publish to origin.
run_becket create parent --repos service >/dev/null 2>&1
commit_file "$(wt parent service)" README.md "PARENT-A" "parent: A"
run_becket push parent >/dev/null 2>&1

# Child stacked on parent (base = A); commit B editing that same line.
run_becket create child --stacked-on parent --repos service >/dev/null 2>&1
commit_file "$(wt child service)" README.md "CHILD-B" "child: B"

# Parent advances to C on the same line, republished → child's B will conflict.
commit_file "$(wt parent service)" README.md "PARENT-C" "parent: C"
run_becket push parent >/dev/null 2>&1

note "restack reports the rebase conflict and how to resolve it"
run_becket restack child
