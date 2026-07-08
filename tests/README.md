# becket tests

becket's test suite is **Go-native**. There are two layers:

1. **Per-package unit tests** (`internal/*/...*_test.go`) — fast, in-process
   tests of pure helpers and data types (config discovery, git wrappers,
   deterministic JSON formatting, render helpers, workspace manifests, CLI
   string utilities).
2. **End-to-end scenarios** (`tests/testscripts/*.txtar`, driven by
   `main_test.go`) — black-box, **golden-master** scripts built on
   [`rogpeppe/go-internal/testscript`][testscript]. They build real git
   fixtures and drive the compiled `becket` as a subprocess, locking its
   *observable* behaviour: stdout, stderr, exit codes, and on-disk state.

[testscript]: https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript

## Running

```bash
go test ./...                              # everything (unit + E2E)
make test                                  # same thing
go test ./... -cover                       # with per-package coverage
go test . -run TestScripts                 # just the E2E scenarios
go test . -run TestScripts/02_lifecycle    # a single scenario
go test . -run TestScripts -update         # regenerate testscript goldens
```

There is no separate build step and no external test framework: `go test`
compiles the test binary, and the testscript harness re-execs that same binary
as `becket` for every `exec becket ...` in a script (see the `TestMain` /
`RunMain` wiring in `main_test.go`). The JSON schemas are `//go:embed`-ed into
the binary, so the subprocess is fully self-contained.

## Requirements

`git` must be on `$PATH` (the E2E scenarios and the `internal/git` tests use a
real git; the git tests `t.Skip` when git is absent). No `bats`, `perl`, or
other tooling is needed.

## How the E2E scenarios work

- **Isolation.** Each `.txtar` runs in its own `$WORK` sandbox. `Setup`
  (`setupEnv` in `main_test.go`) pins a hermetic environment: a private
  `$HOME`/`.gitconfig`, `XDG_DATA_HOME` under `$WORK`, fixed git identity and
  author/committer dates, `TZ=UTC`, `LC_ALL=C`, `GIT_CONFIG_NOSYSTEM=1`, and no
  `EDITOR`/`VISUAL`/`TMUX`. Nothing touches your real environment.
- **Offline git.** Fixtures build each repo with a local **bare repo as its
  `origin`** (inline `exec git init --bare …` / `clone` / `push`), so
  `worktree`, `fetch`, `rebase`, `push`, and `log origin/…` all work with zero
  network.
- **One transcript per scenario.** A scenario drives `exec becket …`, copies
  stdout/stderr into scratch files, normalizes them with the custom `sanitize`
  command, then `cmp`s against an in-archive golden (the `-- file --` blocks at
  the bottom of each `.txtar`). On-disk artifacts (`settings.json`, manifests)
  are sanitized and compared the same way.

### Normalization

The `sanitize` command (defined in `main_test.go`) replaces everything that
legitimately varies run-to-run with a stable placeholder before `cmp`:

| Placeholder | Replaces                                                       |
|-------------|----------------------------------------------------------------|
| `<SANDBOX>` | the `$WORK` sandbox path (and its macOS `/private/var` realpath) |
| `<DATE>`    | `YYYY-MM-DD` dates (e.g. manifest `created`)                   |
| `<TS>`      | ISO-8601 timestamps (e.g. status `updatedAt`)                  |
| `<RELTIME>` | git relative times (`335w ago`, `3 days ago`)                  |
| `<VERSION>` | the version token on the `becket v…` line                      |
| `<SHA>`     | git object ids                                                 |

`sanitize -squeeze` additionally collapses runs of spaces (used where git's
column padding depends on the now-normalized path width).

If a scenario starts flaking, suspect a new unnormalized value first; regenerate
with `go test . -run TestScripts -update` and eyeball the diff.

## E2E scenario coverage

| Scenario             | Commands exercised |
|----------------------|--------------------|
| `00_static`          | `version`, `help` (+ default), `shell-init`, unknown-command + no-config errors |
| `01_init`            | `init` (empty dir, repo discovery, refuse-clobber) |
| `02_lifecycle`       | `create`, `list` (+ json), `status` (+ json), `add`, `desc`, `teardown --delete-branches` |
| `03_stacks`          | `create --stacked-on`, stack-aware `list`/`status`, `restack`, guard errors |
| `04_status_desc`     | `status set`/`clear`, `desc` (explicit id + detect-from-CWD) |
| `05_sync`            | `sync` (no-op, picks up upstream commits, refuses stacked) |
| `06_push_log`        | `push`, `log` |
| `07_stats`           | `stats` |
| `08_upgrade`         | `upgrade` (migrate old config, idempotent, manifests) |
| `09_adopt`           | `adopt` (refuse-on-base, adopt existing branch) |
| `10_shell`           | `shell` (by id, detect-from-CWD, not-found) |
| `11_setup_files`     | `files` copy (+ missing), `setup` (env injected), `create --setup` |
| `12_flags`           | `create --base`, `teardown` (keep branches), `add` from CWD |
| `13_adopt_dirty`     | `adopt` auto-stash + restore into worktree |
| `14_sync_conflict`   | `sync` rebase conflict |
| `15_restack_conflict`| `restack` rebase conflict + resolution hints |
| `16_discovery`       | `init` default-base detection (main vs master) |
| `17_stack_fallback`  | stacked child where parent lacks a repo → base fallback warning |
| `18_dev`             | `dev` (multiplexed run exits cleanly; `--repo` raw passthrough) |
| `20_create_branch`   | `create --branch` (remote-only fetch+track, local-exists, missing-branch error, `--desc` guard) |

Every dispatched command is exercised **except `pr`**, and most have
multiple branches covered.

### Not covered

- **`dev`** — the clean-exit and `--repo` paths are covered by `18_dev` (with a
  fast-exiting dev command). The interactive parts — multiplexed output ordering
  across several long-running processes and Ctrl-C shutdown — aren't
  deterministically golden-testable; verify those by hand in a real workspace.
- **`pr`** — requires the `gh` CLI and a real GitHub remote/auth; likewise only
  coverable with a fake `gh`.
- **The interactive repo picker** in `create`/`adopt` — needs a real TTY on
  stdin, so it can't be golden-tested. The non-TTY default (select all repos) is
  covered instead.

### Git-version sensitivity

The conflict scenarios (`14`, `15`) capture **git's own** rebase and hint text,
which can change between git versions. They run as part of the normal suite. If
you upgrade git and those two go red with only wording differences, regenerate
with `go test . -run TestScripts -update` and eyeball the diff.

## History

This suite began as the **migration contract** for the bash → Go + Cobra
rewrite, first as a bash golden-master runner (`run.sh` + `lib/` + `scenarios/`
+ `golden/`), then ported to Go testscript. The bash runner has been retired (it
lives in git history); the Go binary is the sole implementation and the
testscript goldens track it.
