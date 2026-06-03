# becket characterization tests

A black-box **golden-master** suite that locks the *observable* behaviour of
`becket` — stdout, stderr, exit codes, and the on-disk state each command
produces. Its job is to be the **migration contract**: capture exactly what the
current bash script does today, then prove a reimplementation (the planned
Go + Cobra port) behaves identically by running the *same* suite against it.

> Phase 1 of the migration plan. These are the first tests `becket` has ever
> had — they're worth keeping regardless of whether the migration proceeds.

## Running

```bash
tests/run.sh                 # run all scenarios, diff against goldens
tests/run.sh --update        # (re)generate goldens from current behaviour
tests/run.sh --keep          # keep sandboxes for debugging (prints their paths)
tests/run.sh lifecycle       # only scenarios whose name matches the filter
```

Exit status is `0` when every scenario matches its golden, `1` otherwise (with a
unified diff per failure).

### Validating the future Go binary

The suite drives whatever `$BECKET_BIN` points at, as a black box:

```bash
BECKET_BIN=./becket-go tests/run.sh
```

A self-contained binary is run in place (no staging — see below). Same script,
same args, same goldens: a green run is the equivalence proof.

## Requirements

`bash` (3.2+, the macOS system bash), `git`, `python3` (becket uses it for JSON),
and `perl` (used by the runner for normalization). No `bats` or other test
framework — the runner is plain bash.

## How it works

- **Isolation.** Each scenario runs in its own throwaway sandbox under `$TMPDIR`
  with `HOME`, `XDG_DATA_HOME`, and a private `~/.gitconfig` pointed inside it.
  Git identity, dates, `TZ`, and `LC_ALL` are pinned, and the system/global git
  config is ignored — nothing touches your real environment.
- **Offline git.** Fixtures build each repo with a local **bare repo as its
  `origin`**, so `worktree`, `fetch`, `rebase`, `push`, and `log origin/..`
  all work exactly as against a real remote, with zero network.
- **Install staging.** When testing this repo's `bin/becket`, the runner stages
  a `make install` layout (`$SANDBOX/opt/{bin,share/becket}`) so the script can
  locate its schema dir like a real install. This also exercises the
  schema-copy paths in `init`/`upgrade` that are otherwise dead when running
  uninstalled. (Skipped when `$BECKET_BIN` is overridden.)
- **One transcript per scenario.** A scenario drives a sequence of commands via
  `run_becket` and interleaves state snapshots (`show_manifest`, `show_tree`,
  `show_branches`, `show_worktrees`, `show_settings`). Its full stdout is the
  golden.

### Normalization

Everything that legitimately varies run-to-run is replaced with a stable
placeholder before diffing (see `normalize()` in `run.sh`):

| Placeholder  | Replaces                                              |
|--------------|-------------------------------------------------------|
| `<SANDBOX>`  | the sandbox path (and its macOS `/private/var` realpath) |
| `<PREFIX>`   | the binary's install prefix                           |
| `<DATE>`     | `YYYY-MM-DD` dates (e.g. manifest `created`)          |
| `<TS>`       | ISO-8601 timestamps (e.g. status `updatedAt`)         |
| `<RELTIME>`  | git relative times (`335w ago`, `3 days ago`)         |
| `<SHA>`      | git object ids                                        |

If a scenario starts flaking, suspect a new unnormalized value first.

## Coverage

| Scenario            | Commands exercised |
|---------------------|--------------------|
| `00_static`         | `version`, `help` (+ default), `shell-init`, unknown-command + no-config errors |
| `01_init`           | `init` (empty dir, repo discovery, refuse-clobber) |
| `02_lifecycle`      | `create`, `list` (+ json), `status` (+ json), `add`, `desc`, `teardown --delete-branches` |
| `03_stacks`         | `create --stacked-on`, stack-aware `list`/`status`, `restack`, guard errors |
| `04_status_desc`    | `status set`/`clear`, `desc` (explicit id + detect-from-CWD) |
| `05_sync`           | `sync` (no-op, picks up upstream commits, refuses stacked) |
| `06_push_log`       | `push`, `log` |
| `07_stats`          | `stats` |
| `08_upgrade`        | `upgrade` (migrate old config, idempotent, manifests) |
| `09_adopt`          | `adopt` (refuse-on-base, adopt existing branch) |
| `10_shell`          | `shell` (by id, detect-from-CWD, not-found) |
| `11_setup_files`    | `files` copy (+ missing), `setup` (env injected), `create --setup` (exit 0 path) |
| `12_flags`          | `create --base`, `teardown` (keep branches), `add` from CWD |
| `13_adopt_dirty`    | `adopt` auto-stash + restore into worktree |
| `14_sync_conflict`  | `sync` rebase conflict |
| `15_restack_conflict`| `restack` rebase conflict + resolution hints |
| `16_discovery`      | `init` default-base detection (main vs master) |
| `17_stack_fallback` | stacked child where parent lacks a repo → base fallback warning |

Every dispatched command is now exercised **except `dev` and `pr`**, and most
have multiple branches covered.

### Not covered

- **`dev`** — requires `tmux` and a TTY; spawns a persistent session. Coverable
  only by stubbing a fake `tmux` on `$PATH` (deliberately omitted — the stub
  would make the test less faithful than it looks).
- **`pr`** — requires the `gh` CLI and a real GitHub remote/auth; likewise only
  coverable with a fake `gh` on `$PATH`.
- **The interactive repo picker** in `create`/`adopt` — needs a real TTY on
  stdin, so it can't be golden-tested. The non-TTY default (select all repos) is
  covered instead.
- **`status` of a missing worktree** (worktree dir deleted out from under a
  workspace) — minor branch, not yet exercised.

## Captured quirks — golden ≠ gospel

The goldens record what becket does **today**, bugs included. A diff is a
decision point, not an automatic failure. Known captured bug:

- **`create` and `adopt` exit `1` even on success** when the platform config has
  no `files` array. Cause: `for file in "${files[@]}"` over an empty array under
  `set -u` is an "unbound variable" error on **bash 3.2** (the macOS default
  shell). The workspace, manifest, and worktrees are still created — only the
  exit code (and the file-copy/`--setup` steps after it) are lost. See
  `bin/becket:588` (`create`) and `:781` (`adopt`).

  The Go port should **fix** this (exit `0`). When it does, regenerate the
  affected goldens with `--update` — they currently serve as the "before".

Also note: the conflict scenarios (`14`, `15`) capture **git's own** rebase and
hint text, which changes between git versions. If you upgrade git and those two
go red with only wording differences, `--update` and eyeball the diff.
