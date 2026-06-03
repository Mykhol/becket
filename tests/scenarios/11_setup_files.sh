# shellcheck shell=bash
# Platform `files` copy + `setup` commands + the `create --setup` flag.
# A non-empty `files` array also makes create reach exit 0 (avoiding the empty
# files[@] bug), so this is also the one create path that completes cleanly.
source "$(dirname "$0")/../lib/harness.sh"

setup_platform service

# Replace the init-generated config with one that has files + setup + env.
cat >"$PLATFORM/.becket/settings.json" <<'JSON'
{
  "$schema": "./settings.schema.json",
  "repos": {
    "service": {
      "path": "./service",
      "defaultBase": "main",
      "setup": ["echo configuring service", "echo greeting=$GREETING"],
      "env": { "GREETING": "hi" }
    }
  },
  "branchPrefix": "feature/",
  "files": ["shared.txt", "missing.txt"]
}
JSON
printf 'shared platform file\n' >"$PLATFORM/shared.txt"   # "missing.txt" intentionally absent

note "create --setup: copies platform files (warns on missing), then runs setup"
run_becket create feat --repos service --setup
show_tree

note "standalone setup re-runs the configured commands with env injected"
run_becket setup feat
