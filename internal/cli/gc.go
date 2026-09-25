package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

func newGCCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gc [options]",
		Short: "Remove merged/idle workspaces and prune stale dependency dirs",
		Long: `Dry-run by default: prints what would happen and changes nothing.

A workspace is removed when it is safe (no unpushed or uncommitted work, not
the current directory, not another workspace's stack parent) and eligible
(its branch is merged/closed on GitHub, it has no commits of its own past its
base, or its id matches a disposable pattern and it has sat idle). A kept
workspace that has sat idle past the dependency threshold has its configured
dependency directories (node_modules, .venv, ...) deleted instead, so
'becket deps' reinstalls them on next use.

Options:
  --apply               Execute the plan instead of only printing it
  --idle-days N         Override settings.gc.idleDays (default 3)
  --deps-idle-days N    Override settings.gc.depsIdleDays (default 2)`,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if helpRequested(cmd, args) {
				return
			}
			runGC(args)
		},
	}
}

// gcLine is one reported action: "remove"/"prune" (also acted on with
// --apply) or "keep" (informational only — an eligible-for-removal workspace
// blocked by a safety rule).
type gcLine struct {
	action string
	id     string
	reason string
}

func runGC(args []string) {
	apply := false
	var idleFlag, depsIdleFlag string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--apply":
			apply = true
		case "--idle-days":
			idleFlag, i = next(args, i), i+1
		case "--deps-idle-days":
			depsIdleFlag, i = next(args, i), i+1
		default:
			render.Die("Unknown option: %s", a)
		}
	}

	p := loadPlatform()
	idleDays, depsIdleDays, disposable := effectiveGCSettings(p.Settings.GC, idleFlag, depsIdleFlag)

	// Optional git locks (index refresh on status/rev-list) can bump the
	// index file's mtime, which lastActivity reads — disable them for every
	// git call gc makes so the activity signal stays honest.
	_ = os.Setenv("GIT_OPTIONAL_LOCKS", "0")

	now := gcNow()
	cwd, _ := os.Getwd()

	type loadedWS struct {
		id string
		m  *workspace.Manifest
		ws string
	}
	var all []loadedWS
	stackParents := map[string]bool{}
	for _, mp := range workspace.List(p.WorkspacesDir) {
		m, err := workspace.Load(mp)
		if err != nil {
			continue
		}
		all = append(all, loadedWS{id: m.ID, m: m, ws: filepath.Dir(mp)})
		if m.StackParent != "" {
			stackParents[m.StackParent] = true
		}
	}

	var lines []gcLine
	toRemove, toPrune := 0, 0
	removedCount, prunedCount := 0, 0
	failed := false

	for _, w := range all {
		repos := gatherGCRepos(w.ws, w.m)
		activity := lastActivity(w.ws, w.m)
		idle := int(now.Sub(activity).Hours() / 24)
		if idle < 0 {
			idle = 0
		}

		verdict := decideGCRemoval(cwd, w.ws, stackParents[w.id], repos, idle, idleDays, w.id, disposable)

		removed := false
		switch {
		case verdict.remove:
			lines = append(lines, gcLine{"remove", w.id, verdict.reason})
			toRemove++
			if apply {
				if err := teardownWorkspace(p, w.id, w.m, true); err != nil {
					render.Warn("Could not remove workspace %s: %v", w.id, err)
					failed = true
				} else {
					removedCount++
					removed = true
				}
			}
		case verdict.blocked:
			lines = append(lines, gcLine{"keep", w.id, verdict.reason})
		}

		if !removed {
			targets := planDepsPrune(p, w.ws, w.m, idle, depsIdleDays)
			if len(targets) > 0 {
				var labels []string
				for _, t := range targets {
					labels = append(labels, t.labels...)
				}
				reason := fmt.Sprintf("deps idle %dd: %s", idle, strings.Join(dedupeStrings(labels), ", "))
				lines = append(lines, gcLine{"prune", w.id, reason})
				toPrune++
				if apply {
					if applyDepsPrune(w.ws, targets) {
						prunedCount++
					} else {
						failed = true
					}
				}
			}
		}
	}

	printGCPlan(lines)
	if apply {
		fmt.Printf("%d removed, %d pruned.\n", removedCount, prunedCount)
	} else {
		fmt.Printf("%d to remove, %d to prune — dry run, pass --apply\n", toRemove, toPrune)
	}
	if failed {
		os.Exit(1)
	}
}

// effectiveGCSettings resolves idle thresholds and the disposable-id glob
// list from settings.gc, applying the documented defaults, then the CLI flag
// overrides (which win over both).
func effectiveGCSettings(s *config.GCConfig, idleFlag, depsIdleFlag string) (idleDays, depsIdleDays int, disposable []string) {
	idleDays, depsIdleDays = 3, 2
	disposable = []string{"spike-*", "*review*"}
	if s != nil {
		if s.IdleDays != nil {
			idleDays = *s.IdleDays
		}
		if s.DepsIdleDays != nil {
			depsIdleDays = *s.DepsIdleDays
		}
		if len(s.Disposable) > 0 {
			disposable = s.Disposable
		}
	}
	if idleFlag != "" {
		if n, err := strconv.Atoi(idleFlag); err == nil {
			idleDays = n
		}
	}
	if depsIdleFlag != "" {
		if n, err := strconv.Atoi(depsIdleFlag); err == nil {
			depsIdleDays = n
		}
	}
	return
}

// gcNow returns "now" for idle calculations, honoring BECKET_NOW (RFC3339) so
// tests can make workspaces idle deterministically.
func gcNow() time.Time {
	if v := os.Getenv("BECKET_NOW"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Now()
}

// gcRepoInfo is one manifest repo's on-disk state, gathered once per
// workspace and reused across the safety and eligibility checks.
type gcRepoInfo struct {
	name   string
	wt     string
	exists bool
	entry  workspace.RepoEntry
}

func gatherGCRepos(ws string, m *workspace.Manifest) []gcRepoInfo {
	repos := make([]gcRepoInfo, 0, len(m.Order))
	for _, name := range m.Order {
		wt := filepath.Join(ws, name)
		fi, err := os.Stat(wt)
		repos = append(repos, gcRepoInfo{
			name: name, wt: wt, exists: err == nil && fi.IsDir(), entry: m.Repos[name],
		})
	}
	return repos
}

// lastActivity is the newest of: the manifest's mtime, each repo worktree's
// gitdir HEAD/index mtimes, and each repo's HEAD commit's committer time.
// Computed from file mtimes and git-log (which reads objects, not the index)
// only — callers must call this before any git status/rev-list on the same
// worktrees, which (even read-only) can otherwise refresh the index and bump
// its mtime.
func lastActivity(ws string, m *workspace.Manifest) time.Time {
	var latest time.Time
	bump := func(t time.Time) {
		if t.After(latest) {
			latest = t
		}
	}
	if fi, err := os.Stat(filepath.Join(ws, ".becket.json")); err == nil {
		bump(fi.ModTime())
	}
	for _, name := range m.Order {
		wt := filepath.Join(ws, name)
		if gd, err := worktreeGitDir(wt); err == nil {
			if fi, err := os.Stat(filepath.Join(gd, "HEAD")); err == nil {
				bump(fi.ModTime())
			}
			if fi, err := os.Stat(filepath.Join(gd, "index")); err == nil {
				bump(fi.ModTime())
			}
		}
		if iso := git.LastCommitISO(wt, "HEAD"); iso != "" {
			if t, err := time.Parse(time.RFC3339, iso); err == nil {
				bump(t)
			}
		}
	}
	return latest
}

// worktreeGitDir reads a worktree's ".git" file (format: "gitdir: <path>") and
// returns the path it points at, without running git.
func worktreeGitDir(wt string) (string, error) {
	data, err := os.ReadFile(filepath.Join(wt, ".git"))
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(s, prefix) {
		return "", fmt.Errorf("unrecognized .git file in %s", wt)
	}
	return strings.TrimPrefix(s, prefix), nil
}

// gcVerdict is the removal decision for one workspace. blocked marks a
// workspace that qualified for removal but was kept by a safety rule, so the
// plan can say why.
type gcVerdict struct {
	remove  bool
	blocked bool
	reason  string
}

// decideGCRemoval applies the local safety rules first and only asks GitHub
// about workspaces that pass them, since gh costs a network round trip per repo.
// A merged PR stands in for "pushed": squash merges usually delete the remote
// branch, which leaves the local commits reachable from no remote ref.
func decideGCRemoval(cwd, ws string, isStackParent bool, repos []gcRepoInfo, idle, idleDays int, id string, disposable []string) gcVerdict {
	localReason := localEligibility(repos, idle, idleDays, id, disposable)

	if blocker := localSafetyBlocker(cwd, ws, isStackParent, repos); blocker != "" {
		return gcVerdict{blocked: localReason != "", reason: blocker}
	}

	unpushed := unpushedRepos(repos)
	if localReason != "" && len(unpushed) == 0 {
		return gcVerdict{remove: true, reason: localReason}
	}

	if prs, allClosed := lookupClosedPRs(repos); allClosed && coveredByPRs(repos, unpushed, prs) {
		return gcVerdict{remove: true, reason: "merged"}
	}

	if localReason != "" {
		return gcVerdict{blocked: true, reason: "unpushed: " + unpushed[0].name}
	}
	return gcVerdict{}
}

func localSafetyBlocker(cwd, ws string, isStackParent bool, repos []gcRepoInfo) string {
	if cwd != "" && pathContains(ws, cwd) {
		return "in use (cwd)"
	}
	if isStackParent {
		return "has stack children"
	}
	for _, r := range repos {
		if !r.exists {
			return "missing worktree: " + r.name
		}
	}
	for _, r := range repos {
		out, err := git.Output(r.wt, "status", "--porcelain")
		if err != nil || out != "" {
			return "dirty: " + r.name
		}
	}
	return ""
}

func unpushedRepos(repos []gcRepoInfo) []gcRepoInfo {
	var out []gcRepoInfo
	for _, r := range repos {
		if n, ok := revListCount(r.wt, "HEAD", "--not", "--remotes"); !ok || n != 0 {
			out = append(out, r)
		}
	}
	return out
}

// localEligibility returns why a workspace qualifies for removal without
// asking GitHub (no-work or disposable, both gated on idleness), or "".
func localEligibility(repos []gcRepoInfo, idle, idleDays int, id string, disposable []string) string {
	if len(repos) == 0 || idle < idleDays {
		return ""
	}
	for _, r := range repos {
		if !r.exists {
			return ""
		}
	}
	noWork := true
	for _, r := range repos {
		base := "origin/" + r.entry.Base
		if !git.Verify(r.wt, base) {
			base = r.entry.Base
		}
		if n, ok := revListCount(r.wt, "HEAD", "--not", base); !ok || n != 0 {
			noWork = false
			break
		}
	}
	if noWork {
		return fmt.Sprintf("no-work, idle %dd", idle)
	}
	if matchesAnyGlob(id, disposable) {
		return fmt.Sprintf("disposable, idle %dd", idle)
	}
	return ""
}

type ghPR struct {
	State      string `json:"state"`
	HeadRefOid string `json:"headRefOid"`
}

// lookupClosedPRs returns each repo's most recent PR for its branch, and
// whether at least one PR exists and every found PR is merged or closed.
func lookupClosedPRs(repos []gcRepoInfo) (map[string]ghPR, bool) {
	prs := map[string]ghPR{}
	for _, r := range repos {
		pr, ok := ghLatestPR(r.wt, r.entry.Branch)
		if !ok {
			continue
		}
		if pr.State != "MERGED" && pr.State != "CLOSED" {
			return nil, false
		}
		prs[r.name] = pr
	}
	return prs, len(prs) > 0
}

// coveredByPRs reports whether every unpushed repo's HEAD is exactly the head
// its closed PR recorded, so GitHub still holds those commits.
func coveredByPRs(repos, unpushed []gcRepoInfo, prs map[string]ghPR) bool {
	for _, r := range unpushed {
		pr, ok := prs[r.name]
		if !ok {
			return false
		}
		head, err := git.Output(r.wt, "rev-parse", "HEAD")
		if err != nil || head != pr.HeadRefOid {
			return false
		}
	}
	return true
}

func revListCount(wt string, args ...string) (int, bool) {
	out, err := git.Output(wt, append([]string{"rev-list", "--count"}, args...)...)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, false
	}
	return n, true
}

func matchesAnyGlob(id string, patterns []string) bool {
	for _, pat := range patterns {
		if ok, err := path.Match(pat, id); err == nil && ok {
			return true
		}
	}
	return false
}

// ghLatestPR queries the most recent PR for branch in the repo at wt. ok is
// false when gh isn't installed, the call fails (e.g. a non-GitHub remote), or
// there is no PR — all "unknown", not an error.
func ghLatestPR(wt, branch string) (ghPR, bool) {
	if _, err := exec.LookPath("gh"); err != nil {
		return ghPR{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "pr", "list",
		"--head", branch, "--state", "all", "--json", "state,headRefOid", "--limit", "1")
	cmd.Dir = wt
	out, err := cmd.Output()
	if err != nil {
		return ghPR{}, false
	}
	var prs []ghPR
	if json.Unmarshal(out, &prs) != nil || len(prs) == 0 {
		return ghPR{}, false
	}
	return prs[0], true
}

// depsPruneTarget is one repo's dependency directories to delete for an idle
// (but kept) workspace.
type depsPruneTarget struct {
	repo   string
	dirs   []string // absolute paths to remove
	labels []string // the configured Dirs patterns that matched, for display
}

// planDepsPrune finds, for each repo with a deps block, the configured Dirs
// that currently exist — refusing (with a warning) any match that escapes the
// worktree or is tracked by git.
func planDepsPrune(p *config.Platform, ws string, m *workspace.Manifest, idle, depsIdleDays int) []depsPruneTarget {
	if idle < depsIdleDays {
		return nil
	}
	var out []depsPruneTarget
	for _, repo := range m.Order {
		rc := p.Settings.Repos[repo]
		if rc.Deps == nil || len(rc.Deps.Dirs) == 0 {
			continue
		}
		wt := filepath.Join(ws, repo)
		if fi, err := os.Stat(wt); err != nil || !fi.IsDir() {
			continue
		}
		var dirs, labels []string
		for _, pattern := range rc.Deps.Dirs {
			matches, _ := filepath.Glob(filepath.Join(wt, pattern))
			found := false
			for _, m := range matches {
				if !safeToPrune(wt, m) {
					render.Warn("Refusing to prune %s (escapes worktree or tracked by git).", m)
					continue
				}
				dirs = append(dirs, m)
				found = true
			}
			if found {
				labels = append(labels, pattern)
			}
		}
		if len(dirs) > 0 {
			out = append(out, depsPruneTarget{repo: repo, dirs: dirs, labels: labels})
		}
	}
	return out
}

// safeToPrune reports whether match is strictly inside wt and holds no
// git-tracked files.
func safeToPrune(wt, match string) bool {
	wtAbs, err1 := filepath.Abs(wt)
	matchAbs, err2 := filepath.Abs(match)
	if err1 != nil || err2 != nil {
		return false
	}
	if matchAbs == wtAbs || !strings.HasPrefix(matchAbs, wtAbs+string(filepath.Separator)) {
		return false
	}
	rel, err := filepath.Rel(wt, match)
	if err != nil {
		return false
	}
	out, err := git.Output(wt, "ls-files", "--", rel)
	if err != nil || out != "" {
		return false
	}
	return true
}

// applyDepsPrune deletes the planned directories and each pruned repo's deps
// stamp (so 'becket deps' reinstalls). Returns false if any deletion failed.
func applyDepsPrune(ws string, targets []depsPruneTarget) bool {
	ok := true
	for _, t := range targets {
		for _, d := range t.dirs {
			if err := os.RemoveAll(d); err != nil {
				render.Warn("Could not remove %s: %v", d, err)
				ok = false
			}
		}
		_ = os.Remove(depsStampPath(ws, t.repo))
	}
	return ok
}

func dedupeStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func printGCPlan(lines []gcLine) {
	if len(lines) == 0 {
		return
	}
	idW := 0
	for _, l := range lines {
		idW = max(idW, len(l.id))
	}
	idW += 2
	for _, l := range lines {
		fmt.Printf("%s%s%s\n", render.PadRight(l.action, 8), render.PadRight(l.id, idW), l.reason)
	}
	fmt.Println()
}
