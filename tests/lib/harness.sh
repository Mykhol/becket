# shellcheck shell=bash
# ──────────────────────────────────────────────────────────────────────────────
# becket characterization-test harness
#
# Sourced by every scenario in tests/scenarios/. Provides:
#   - a hermetic sandbox ($SANDBOX) with isolated HOME / git config
#   - fixture builders for a platform dir of git repos (each with a local
#     bare "origin" so fetch/push/rebase work fully offline)
#   - run_becket: invokes $BECKET_BIN, capturing exit code + stdout + stderr
#     into a stable, labelled transcript block
#   - state-snapshot helpers (settings, manifests, branches, worktrees, tree)
#
# This file is implementation-agnostic: it drives the binary named by
# $BECKET_BIN as a black box. The SAME suite runs against the bash script today
# and the Go binary tomorrow — that swap is the entire point (the migration
# contract). Nothing here may reach into becket's internals.
#
# Targets bash 3.2 (the macOS system bash), matching becket itself: indexed
# arrays only, no `declare -A`, no `mapfile`.
# ──────────────────────────────────────────────────────────────────────────────

# run.sh exports: SANDBOX, BECKET_BIN. Everything else derives from those.
: "${SANDBOX:?harness must be sourced by run.sh (SANDBOX unset)}"
: "${BECKET_BIN:?harness must be sourced by run.sh (BECKET_BIN unset)}"

# Stable, isolated environment ------------------------------------------------
export HOME="$SANDBOX/home"
export XDG_DATA_HOME="$HOME/.local/share"
export TZ="UTC"
export LC_ALL="C"
# Make git fully deterministic and independent of the developer's real config.
export GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME="Becket Tester"   GIT_AUTHOR_EMAIL="test@becket.invalid"
export GIT_COMMITTER_NAME="Becket Tester" GIT_COMMITTER_EMAIL="test@becket.invalid"
export GIT_AUTHOR_DATE="2020-01-01T00:00:00Z"
export GIT_COMMITTER_DATE="2020-01-01T00:00:00Z"
# Drop anything that could make becket open an editor or attach to a terminal.
unset EDITOR VISUAL TMUX

PLATFORM="$SANDBOX/platform"
ORIGINS="$SANDBOX/origins"

_harness_bootstrap() {
  mkdir -p "$HOME" "$ORIGINS"

  # Stage an install layout ($SANDBOX/opt/{bin,share/becket}) so the binary
  # resolves its schema dir like a real install. run.sh sets BECKET_STAGE=1 only
  # when testing this repo's bash script; a self-contained binary skips this.
  if [ "${BECKET_STAGE:-0}" = "1" ]; then
    local opt="$SANDBOX/opt"
    mkdir -p "$opt/bin" "$opt/share/becket"
    cp "$BECKET_BIN" "$opt/bin/becket"
    chmod +x "$opt/bin/becket"
    cp "$BECKET_SCHEMA_SRC"/*.schema.json "$opt/share/becket/" 2>/dev/null || true
    BECKET_BIN="$opt/bin/becket"
    export BECKET_BIN
  fi

  cat >"$HOME/.gitconfig" <<'EOF'
[user]
	name = Becket Tester
	email = test@becket.invalid
[init]
	defaultBranch = main
[commit]
	gpgsign = false
[core]
	pager = cat
[advice]
	detachedHead = false
EOF
}
_harness_bootstrap

# ── git helper ────────────────────────────────────────────────────────────────
# Quiet git; scenarios never want fixture git chatter in the transcript.
_git() { git "$@" >/dev/null 2>&1; }

# ── Fixture builders ───────────────────────────────────────────────────────────

# mk_platform — create and cd into an empty platform directory.
mk_platform() {
  mkdir -p "$PLATFORM"
  cd "$PLATFORM"
}

# mk_repo <name> — create a sibling repo in the platform dir with a local bare
# origin and a single seeded commit on main, upstream-tracked. This makes
# `git worktree add`, `fetch`, `rebase`, `push`, and `log origin/main..` all
# work offline exactly as they would against a real remote.
mk_repo() {
  local name="$1"
  local bare="$ORIGINS/$name.git"
  local work="$PLATFORM/$name"
  _git init --bare "$bare"
  _git clone "$bare" "$work"
  printf '# %s\n' "$name" >"$work/README.md"
  _git -C "$work" add README.md
  _git -C "$work" commit -m "init $name"
  _git -C "$work" push -u origin main
}

# mk_repo_branch <name> <branch> — like mk_repo, but the repo's only branch
# (and its origin default) is <branch> rather than main. Used to characterize
# init's master-vs-main default-base detection.
mk_repo_branch() {
  local name="$1" branch="$2"
  local bare="$ORIGINS/$name.git" work="$PLATFORM/$name"
  _git init --bare "$bare"
  _git clone "$bare" "$work"
  _git -C "$work" checkout -b "$branch"
  printf '# %s\n' "$name" >"$work/README.md"
  _git -C "$work" add README.md
  _git -C "$work" commit -m "init $name"
  _git -C "$work" push -u origin "$branch"
}

# commit_file <gitdir> <relpath> <content> <message> — stage+commit one file.
commit_file() {
  local dir="$1" rel="$2" content="$3" msg="$4"
  printf '%s\n' "$content" >"$dir/$rel"
  _git -C "$dir" add "$rel"
  _git -C "$dir" commit -m "$msg"
}

# wt <id> <repo> — path to a repo's worktree inside a workspace.
wt() { echo "$PLATFORM/.becket/workspaces/$1/$2"; }

# setup_platform <repo>... — full bootstrap for scenarios that exercise commands
# OTHER than init: build the repos and run `becket init` quietly so the platform
# starts from a known, valid config. init's own output is NOT in the transcript
# here — test init explicitly in the init scenario instead.
setup_platform() {
  mk_platform
  local r
  for r in "$@"; do mk_repo "$r"; done
  "$BECKET_BIN" init </dev/null >/dev/null 2>&1 || true
}

# ── Invocation + capture ────────────────────────────────────────────────────────

# run_becket <args...> — run the binary under test as a black box and append a
# normalised, labelled block to the transcript. stdin is always /dev/null so the
# interactive repo picker takes its non-TTY branch (default: all repos) and never
# blocks. stdout and stderr are captured separately to avoid cross-stream
# interleaving nondeterminism.
run_becket() {
  echo "\$ becket $*"
  local out="$SANDBOX/.stdout" err="$SANDBOX/.stderr" rc
  "$BECKET_BIN" "$@" </dev/null >"$out" 2>"$err"
  rc=$?
  echo "[exit $rc]"
  if [ -s "$out" ]; then echo "--- stdout ---"; cat "$out"; fi
  if [ -s "$err" ]; then echo "--- stderr ---"; cat "$err"; fi
  echo
}

# ── State snapshot helpers ───────────────────────────────────────────────────────
# Each prints a labelled block describing some piece of on-disk state, so the
# golden captures the *result* of a command, not just what it said.

_hr() { echo "# $*"; }

# show_settings — the platform settings.json.
show_settings() {
  _hr "settings.json"
  if [ -f "$PLATFORM/.becket/settings.json" ]; then
    cat "$PLATFORM/.becket/settings.json"; echo
  else
    echo "(absent)"
  fi
  echo
}

# show_manifest <id> — a workspace's .becket.json manifest.
show_manifest() {
  local id="$1" m="$PLATFORM/.becket/workspaces/$1/.becket.json"
  _hr "manifest: $id"
  if [ -f "$m" ]; then cat "$m"; echo; else echo "(absent)"; fi
  echo
}

# show_tree — the shape of the workspaces dir (no descent into worktree
# checkouts, which are full repo clones).
show_tree() {
  _hr "workspaces tree"
  local ws="$PLATFORM/.becket/workspaces"
  if [ -d "$ws" ]; then
    ( cd "$ws" && find . -maxdepth 2 ! -name '.' | LC_ALL=C sort )
  else
    echo "(no workspaces dir)"
  fi
  echo
}

# show_branches <repo> — local branches in a platform repo (reveals the feature
# branches becket created).
show_branches() {
  local repo="$1"
  _hr "branches: $repo"
  git -C "$PLATFORM/$repo" branch --format='%(refname:short)' 2>&1 | LC_ALL=C sort
  echo
}

# show_worktrees <repo> — `git worktree list` for a platform repo. The column
# padding in this output is git-version-dependent, so squeeze runs of spaces to a
# single space for portability (the lines have no meaningful internal spacing);
# paths get normalised away by the runner.
show_worktrees() {
  local repo="$1"
  _hr "worktrees: $repo"
  git -C "$PLATFORM/$repo" worktree list 2>&1 | tr -s ' ' | LC_ALL=C sort
  echo
}

# note <text> — free-form section divider in the transcript.
note() { echo "### $*"; echo; }
