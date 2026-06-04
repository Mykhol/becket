#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────────────────
# becket characterization-test runner
#
# Drives every scenario in tests/scenarios/ as a black box against $BECKET_BIN,
# normalises the transcript (paths, SHAs, dates, times), and diffs it against the
# committed golden file. This locks the *observable* behaviour of becket so a
# reimplementation can be proven equivalent.
#
#   tests/run.sh                 # run all scenarios, diff against goldens
#   tests/run.sh --update        # (re)generate goldens from current behaviour
#   tests/run.sh --keep          # keep sandboxes for debugging (prints paths)
#   tests/run.sh lifecycle       # only scenarios whose name matches the filter
#   BECKET_BIN=./becket-go tests/run.sh   # run the suite against another binary
#
# Exit status: 0 if all scenarios match their golden, 1 otherwise.
# ──────────────────────────────────────────────────────────────────────────────
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TESTS="$ROOT/tests"
SCENARIO_DIR="$TESTS/scenarios"
GOLDEN_DIR="$TESTS/golden"

# The binary under test. Defaults to the bash script in this repo; override to
# point the identical suite at the Go build once it exists.
BECKET_BIN="${BECKET_BIN:-$ROOT/bin/becket}"
case "$BECKET_BIN" in /*) ;; *) BECKET_BIN="$(cd "$(dirname "$BECKET_BIN")" && pwd)/$(basename "$BECKET_BIN")" ;; esac

UPDATE=0
KEEP=0
FILTER=""
for arg in "$@"; do
  case "$arg" in
    --update) UPDATE=1 ;;
    --keep)   KEEP=1 ;;
    --help|-h) sed -n '2,20p' "$0"; exit 0 ;;
    -*) echo "unknown flag: $arg" >&2; exit 2 ;;
    *) FILTER="$arg" ;;
  esac
done

if [ ! -x "$BECKET_BIN" ]; then
  echo "error: BECKET_BIN not executable: $BECKET_BIN" >&2
  exit 2
fi

# Staging: reproduce the `make install` layout ($SANDBOX/opt/{bin,share/becket})
# inside each sandbox and run the binary-under-test from there. This gives a
# stable, normalizable install prefix (so shell-init output matches across the
# bash and Go binaries), lets the bash script find its schema dir, and exercises
# the schema-copy paths. A self-contained binary (Go) ignores the staged schemas.
export BECKET_STAGE=1
export BECKET_SCHEMA_SRC="$ROOT/schema"

mkdir -p "$GOLDEN_DIR"

# ── Normalisation ───────────────────────────────────────────────────────────────
# Replace everything that legitimately varies run-to-run with stable placeholders.
# Order matters: paths first, then ISO timestamps before bare dates, then SHAs and
# relative times. SANDBOX is matched both as given and via its realpath, because
# macOS resolves $TMPDIR through /private/var and git prints realpaths.
normalize() {
  # Paths are passed via the environment and quotemeta'd, so regex-special
  # characters in a temp path (`.`, `+`, …) can never corrupt the pattern.
  SANDBOX="$1" SANDBOX_REAL="$2" PREFIX="$3" perl -pe '
    BEGIN {
      $s  = quotemeta $ENV{SANDBOX};
      $sr = quotemeta $ENV{SANDBOX_REAL};
      $p  = quotemeta $ENV{PREFIX};
    }
    s/$sr/<SANDBOX>/g;
    s/$s/<SANDBOX>/g;
    s/$p/<PREFIX>/g if length $ENV{PREFIX};
    s/\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z/<TS>/g;       # ISO timestamps
    s/\d{4}-\d{2}-\d{2}/<DATE>/g;                        # bare dates
    s/\b\d+ (?:second|minute|hour|day|week|month|year)s? ago\b/<RELTIME>/g;
    s/\b\d+[smhdwy] ago\b/<RELTIME>/g;                   # git short relative ("335w ago")
    s/\b[0-9a-f]{7,40}\b/<SHA>/g;                        # git object ids
  '
}

# ── Run one scenario ─────────────────────────────────────────────────────────────
PASS=0; FAIL=0; UPDATED=0
FAILED_NAMES=""

run_one() {
  local file="$1"
  local name; name="$(basename "$file" .sh)"
  local golden="$GOLDEN_DIR/$name.txt"

  local sandbox; sandbox="$(mktemp -d "${TMPDIR:-/tmp}/becket-test.XXXXXX")"
  local sandbox_real; sandbox_real="$(cd "$sandbox" && pwd -P)"

  # Each scenario runs in its own process with a fresh sandbox.
  local raw; raw="$(
    export SANDBOX="$sandbox" BECKET_BIN BECKET_STAGE BECKET_SCHEMA_SRC
    bash "$file" 2>&1
  )"
  local actual; actual="$(printf '%s\n' "$raw" | normalize "$sandbox" "$sandbox_real" "${BECKET_BIN%/bin/becket}")"

  if [ "$KEEP" -eq 1 ]; then echo "  (kept sandbox: $sandbox)"; else rm -rf "$sandbox"; fi

  if [ "$UPDATE" -eq 1 ]; then
    printf '%s\n' "$actual" >"$golden"
    echo "updated  $name"
    UPDATED=$((UPDATED+1))
    return
  fi

  if [ ! -f "$golden" ]; then
    echo "MISSING  $name  (no golden — run with --update)"
    FAIL=$((FAIL+1)); FAILED_NAMES="$FAILED_NAMES $name"
    return
  fi

  if printf '%s\n' "$actual" | diff -u "$golden" - >"$sandbox.diff" 2>&1; then
    echo "ok       $name"
    PASS=$((PASS+1))
    rm -f "$sandbox.diff"
  else
    echo "FAIL     $name"
    sed 's/^/    /' "$sandbox.diff"
    rm -f "$sandbox.diff"
    FAIL=$((FAIL+1)); FAILED_NAMES="$FAILED_NAMES $name"
  fi
}

echo "becket: $BECKET_BIN"
[ -n "$FILTER" ] && echo "filter: $FILTER"
echo

ran=0
for file in "$SCENARIO_DIR"/*.sh; do
  [ -e "$file" ] || continue
  name="$(basename "$file" .sh)"
  if [ -n "$FILTER" ]; then case "$name" in *"$FILTER"*) ;; *) continue ;; esac; fi
  ran=$((ran+1))
  run_one "$file"
done

echo
if [ "$ran" -eq 0 ]; then echo "no scenarios matched"; exit 2; fi
if [ "$UPDATE" -eq 1 ]; then echo "updated $UPDATED golden(s)"; exit 0; fi
echo "passed $PASS, failed $FAIL"
[ "$FAIL" -eq 0 ] || { echo "failed:$FAILED_NAMES"; exit 1; }
