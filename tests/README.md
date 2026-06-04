# becket characterization tests

A black-box **golden-master** suite that locks the *observable* behaviour of
`becket` — stdout, stderr, exit codes, and the on-disk state each command
produces. It began as the **migration contract** (prove the Go + Cobra rewrite
matched the original bash script byte-for-byte); that migration is complete, so
it now serves as a **regression suite for the Go binary**, which is the source of
truth.

## Running

```bash
make test                    # build the Go binary and run the suite
tests/run.sh --update        # (re)generate goldens from current behaviour
tests/run.sh --keep          # keep sandboxes for debugging (prints their paths)
tests/run.sh lifecycle       # only scenarios whose name matches the filter
```

The runner drives whatever `$BECKET_BIN` points at as a black box; `make test`
builds `becket-go` and points it there. Exit status is `0` when every scenario
matches its golden, `1` otherwise (with a unified diff per failure).

> The original bash implementation at `bin/becket` is a frozen reference and is
> no longer held to this suite.

## Requirements

The runner is plain bash and needs `git` plus `perl` (for output normalization).
The Go binary under test needs only `git`. No `bats` or other test framework.

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

## Source of truth & history

The goldens track the **Go binary**. During the migration they were kept green
against both the bash script and the Go port (a single golden set proving
equivalence); now that the Go binary is the product, the goldens follow it and
`bin/becket` is no longer tested against them.

The runner stages an install layout per sandbox, so the install prefix (used by
`shell-init`) normalizes deterministically.

### Git-version sensitivity

The conflict scenarios (`14`, `15`) capture **git's own** rebase and hint text,
which changes between git versions. If you upgrade git and those two go red with
only wording differences, `--update` and eyeball the diff. (CI excludes them via
`BECKET_SKIP=conflict` and runs them informationally.)
