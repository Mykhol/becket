# shellcheck shell=bash
# `becket upgrade` — migrate an older config/manifest forward: add the $schema
# reference and fill in default keys, idempotently.
source "$(dirname "$0")/../lib/harness.sh"

mk_platform
mk_repo service

note "an old-style settings.json: no \$schema, no branchPrefix"
mkdir -p "$PLATFORM/.becket"
cat >"$PLATFORM/.becket/settings.json" <<'JSON'
{
  "repos": {
    "service": {
      "path": "./service",
      "defaultBase": "main"
    }
  }
}
JSON
show_settings

note "upgrade adds the schema reference and default keys"
run_becket upgrade
show_settings

note "upgrade is idempotent and also visits workspace manifests"
run_becket create ws --repos service >/dev/null 2>&1
run_becket upgrade
show_manifest ws
