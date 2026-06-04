package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// This file is the Go-native end-to-end test harness for becket, built on
// rogpeppe/go-internal/testscript. It is intended to eventually REPLACE the
// bash characterization suite under tests/ (run.sh + lib/harness.sh +
// scenarios/*.sh + golden/*.txt). The bash suite stays until a later
// integration step confirms parity — do not delete it.
//
// ─────────────────────────────────────────────────────────────────────────────
// How it works
// ─────────────────────────────────────────────────────────────────────────────
//
//   - testscript.RunMain re-executes THIS test binary with os.Args[0]=="becket"
//     for every `exec becket ...` in a script. Because that runs as a separate
//     OS process, becket's main() calling os.Exit(...) is fine, and the
//     //go:embed schemas (compiled from main.go into the test binary) are
//     present in the subprocess.
//
//   - Each .txtar script under tests/testscripts/ is a self-contained scenario:
//     it builds its own git fixtures with `exec git ...` (a bare origin + clone,
//     matching tests/lib/harness.sh) and drives becket as a black box.
//
//   - The Setup func pins a hermetic, deterministic environment per script so
//     output is reproducible (fixed HOME/.gitconfig, fixed git identities and
//     author/committer dates, TZ=UTC, no EDITOR/TMUX, XDG under $WORK).
//
// ─────────────────────────────────────────────────────────────────────────────
// NORMALIZATION (the crux — copy this pattern when porting more scenarios)
// ─────────────────────────────────────────────────────────────────────────────
//
// Real becket output contains values that legitimately vary run-to-run: the
// sandbox path, git object SHAs, dates, ISO timestamps, git short relative
// times ("335w ago"), and the version token. We collapse those to stable
// placeholders, mirroring the perl normalizer in tests/run.sh exactly.
//
// The mechanism is a custom testscript command, `sanitize <file>...`, which
// rewrites those patterns IN PLACE in the given file(s). The canonical scenario
// pattern is:
//
//	exec becket <args>
//	cp stdout out          # testscript's cp can copy the stdout/stderr buffers
//	sanitize out           # normalize dynamic values to placeholders
//	cmp out expected.txt   # compare against the in-archive golden
//
// With `go test ... -run TestScripts -update`, a failing `cmp` whose second
// file lives in the txtar archive rewrites that golden with the (already
// sanitized) actual output — so goldens are regenerated, not hand-edited.
//
// Substitution rules (order matters: paths first, ISO timestamps before bare
// dates, then version, then SHAs last):
//
//	$WORK and its realpath   -> <SANDBOX>   (handled in sanitize via env, since
//	                                          git prints realpaths on macOS)
//	YYYY-MM-DDTHH:MM:SSZ      -> <TS>
//	YYYY-MM-DD                -> <DATE>
//	"<n> {second..year}s ago" -> <RELTIME>
//	"<n>[smhdwy] ago"         -> <RELTIME>   (git short relative form)
//	leading "becket v<...>"   -> "becket v<VERSION>"
//	[0-9a-f]{7,40}            -> <SHA>
//
// becket's version in tests is "dev" (main.version default), so the version
// line normalizes to "becket v<VERSION>" just like a stamped release build.

var (
	reTS      = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)
	reDate    = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	reRelLong = regexp.MustCompile(`\b\d+ (?:second|minute|hour|day|week|month|year)s? ago\b`)
	reRelShrt = regexp.MustCompile(`\b\d+[smhdwy] ago\b`)
	reVersion = regexp.MustCompile(`(?m)^becket v\S+`)
	reSHA     = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	reSpaces  = regexp.MustCompile(` +`)
)

// normalize applies the same substitutions as the perl normalizer in
// tests/run.sh. work is $WORK and workReal is its realpath (macOS resolves
// $TMPDIR through /private/var, and git prints realpaths).
func normalize(s, work, workReal string) string {
	if workReal != "" && workReal != work {
		s = replaceAll(s, workReal, "<SANDBOX>")
	}
	s = replaceAll(s, work, "<SANDBOX>")
	s = reTS.ReplaceAllString(s, "<TS>")
	s = reDate.ReplaceAllString(s, "<DATE>")
	s = reRelLong.ReplaceAllString(s, "<RELTIME>")
	s = reRelShrt.ReplaceAllString(s, "<RELTIME>")
	s = reVersion.ReplaceAllString(s, "becket v<VERSION>")
	s = reSHA.ReplaceAllString(s, "<SHA>")
	return s
}

// replaceAll is strings.ReplaceAll without importing strings just for this.
func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	var b []byte
	for {
		i := indexOf(s, old)
		if i < 0 {
			b = append(b, s...)
			break
		}
		b = append(b, s[:i]...)
		b = append(b, new...)
		s = s[i+len(old):]
	}
	return string(b)
}

func indexOf(s, sub string) int {
	n := len(sub)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		// Re-entrant becket: testscript runs this branch in a child process
		// with os.Args[0]=="becket". main() reads os.Args and os.Exit()s,
		// which is correct in a subprocess. Returning 0 is unreachable.
		"becket": func() int { main(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:           "tests/testscripts",
		UpdateScripts: *update,
		Setup:         setupEnv,
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// sanitize [-squeeze] <file>...: rewrite dynamic values to
			// placeholders in place. See the package doc above for the
			// substitution rules. With -squeeze, runs of spaces are first
			// collapsed to a single space (matching `tr -s ' '` in
			// harness.sh's show_worktrees), needed for git-version-dependent
			// column padding whose width depends on the now-normalized path.
			"sanitize": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg {
					ts.Fatalf("unsupported: ! sanitize")
				}
				squeeze := false
				if len(args) > 0 && args[0] == "-squeeze" {
					squeeze = true
					args = args[1:]
				}
				if len(args) == 0 {
					ts.Fatalf("usage: sanitize [-squeeze] file...")
				}
				work := ts.Getenv("WORK")
				workReal := ts.Getenv("WORK_REAL")
				for _, a := range args {
					p := ts.MkAbs(a)
					data, err := os.ReadFile(p)
					ts.Check(err)
					s := string(data)
					if squeeze {
						s = reSpaces.ReplaceAllString(s, " ")
					}
					out := normalize(s, work, workReal)
					ts.Check(os.WriteFile(p, []byte(out), 0o644))
				}
			},
		},
	})
}

// update mirrors the conventional `-update` flag testscript suites use to
// regenerate goldens: go test ./... -run TestScripts -update.
var update = flag.Bool("update", false, "update testscript golden files")

// setupEnv pins a hermetic, deterministic environment for every script. The
// values match tests/lib/harness.sh so behaviour (and therefore golden output)
// is identical to the bash suite.
func setupEnv(env *testscript.Env) error {
	work := env.WorkDir
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	xdg := filepath.Join(home, ".local", "share")
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		return err
	}

	// A fixed git config so git is fully deterministic and independent of the
	// developer's real config — mirrors _harness_bootstrap's .gitconfig.
	gitconfig := "[user]\n" +
		"\tname = Becket Tester\n" +
		"\temail = test@becket.invalid\n" +
		"[init]\n" +
		"\tdefaultBranch = main\n" +
		"[commit]\n" +
		"\tgpgsign = false\n" +
		"[core]\n" +
		"\tpager = cat\n" +
		"[advice]\n" +
		"\tdetachedHead = false\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o644); err != nil {
		return err
	}

	// WORK_REAL is $WORK resolved through symlinks (macOS /private/var); the
	// sanitize command uses it because git prints realpaths.
	workReal, err := filepath.EvalSymlinks(work)
	if err != nil {
		workReal = work
	}

	env.Setenv("HOME", home)
	env.Setenv("WORK_REAL", workReal)
	env.Setenv("XDG_DATA_HOME", xdg)
	env.Setenv("TZ", "UTC")
	env.Setenv("LC_ALL", "C")
	env.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	env.Setenv("GIT_AUTHOR_NAME", "Becket Tester")
	env.Setenv("GIT_AUTHOR_EMAIL", "test@becket.invalid")
	env.Setenv("GIT_COMMITTER_NAME", "Becket Tester")
	env.Setenv("GIT_COMMITTER_EMAIL", "test@becket.invalid")
	env.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00Z")
	env.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")

	// Drop anything that could make becket open an editor or attach to a
	// terminal (matches harness.sh's `unset EDITOR VISUAL TMUX`).
	env.Setenv("EDITOR", "")
	env.Setenv("VISUAL", "")
	env.Setenv("TMUX", "")
	return nil
}
