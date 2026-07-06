package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/jsonfmt"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

// status JSON shapes (field order mirrors the bash python dicts).
type statusRepo struct {
	Name       string  `json:"name"`
	Branch     string  `json:"branch"`
	Base       string  `json:"base"`
	Status     string  `json:"status"`
	Ahead      *int    `json:"ahead"`
	Behind     *int    `json:"behind"`
	LastCommit *string `json:"lastCommit"`
}

type statusWS struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Created     string            `json:"created"`
	Status      *workspace.Status `json:"status"`
	StackParent string            `json:"stackParent"`
	Repos       []statusRepo      `json:"repos"`
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [id]",
		Short: "Show branch status across repos",
		Long: `Show branch, clean/dirty state, ahead/behind, and last-commit time per repo.

  status [id] [--output json]    Show status (id detected from CWD if omitted)
  status set <text> [--by who]   Set a workspace status note
  status clear [id]              Clear the status note`,
		DisableFlagParsing: true, // route set/clear before any flag handling
		Run: func(cmd *cobra.Command, args []string) {
			if helpRequested(cmd, args) {
				return
			}
			if len(args) > 0 {
				switch args[0] {
				case "set":
					runStatusSet(args[1:])
					return
				case "clear":
					runStatusClear(args[1:])
					return
				}
			}
			// Parse --output|-o and the optional positional id manually.
			output := "table"
			var positional []string
			for i := 0; i < len(args); i++ {
				switch args[i] {
				case "--output", "-o":
					output = next(args, i)
					i++
				default:
					positional = append(positional, args[i])
				}
			}
			runStatus(output, positional)
		},
	}
}

func runStatus(output string, args []string) {
	p := loadPlatform()

	target := arg0(args)
	if target == "" {
		target, _ = workspace.Detect()
	}

	var manifests []string
	if target != "" {
		mp := manifestPath(p, target)
		if fi, err := os.Stat(mp); err != nil || fi.IsDir() {
			render.Die("Workspace '%s' not found.", target)
		}
		manifests = []string{mp}
	} else {
		manifests = workspace.List(p.WorkspacesDir)
	}
	if len(manifests) == 0 {
		render.Die("No workspaces found.")
	}

	var data []statusWS
	for _, mp := range manifests {
		m, err := workspace.Load(mp)
		if err != nil {
			continue
		}
		wsDir := wsPath(p, m.ID)
		ws := statusWS{
			ID: m.ID, Description: m.Description, Created: m.Created,
			Status: m.Status, StackParent: m.StackParent,
		}
		for _, name := range m.Order {
			entry := m.Repos[name]
			wt := wsDir + "/" + name
			if fi, err := os.Stat(wt); err != nil || !fi.IsDir() {
				ws.Repos = append(ws.Repos, statusRepo{Name: name, Status: "missing"})
				continue
			}
			branch := git.CurrentBranch(wt)
			if branch == "" {
				if _, err := git.Output(wt, "branch", "--show-current"); err != nil {
					branch = "detached"
				}
			}
			st := "dirty"
			if git.IsClean(wt) {
				st = "clean"
			}
			base := entry.Base
			// Count against origin/<base> when it exists — create and sync
			// both work against origin, and the local base branch goes stale
			// in a worktree flow (nothing ever pulls it).
			countBase := base
			if git.Verify(wt, "origin/"+base) {
				countBase = "origin/" + base
			}
			ahead := parseCount(git.CountRange(wt, countBase+".."+branch))
			behind := parseCount(git.CountRange(wt, branch+".."+countBase))
			var lc *string
			if iso := git.LastCommitISO(wt, branch); iso != "" {
				lc = &iso
			}
			ws.Repos = append(ws.Repos, statusRepo{
				Name: name, Branch: branch, Base: base, Status: st,
				Ahead: ahead, Behind: behind, LastCommit: lc,
			})
		}
		data = append(data, ws)
	}

	if output == "json" {
		if data == nil {
			data = []statusWS{}
		}
		b, _ := jsonfmt.Indent4(data)
		fmt.Print(string(b))
		return
	}
	renderStatusTable(data)
}

func renderStatusTable(data []statusWS) {
	b, r := "", ""
	if render.IsTTY() {
		b, r = "\033[1m", "\033[0m"
	}
	for _, ws := range data {
		repoW, branchW := len("REPO"), len("BRANCH")
		for _, rp := range ws.Repos {
			repoW = max(repoW, len(rp.Name))
			branchW = max(branchW, len(rp.Branch))
		}
		repoW += 2
		branchW += 2
		statusW, abW := 10, 14

		title := fmt.Sprintf("%sWorkspace: %s%s", b, ws.ID, r)
		if ws.Description != "" {
			title += " — " + ws.Description
		}
		fmt.Println()
		fmt.Println(title)
		if ws.StackParent != "" {
			fmt.Printf("  Stacked on: %s\n", ws.StackParent)
		}
		if ws.Status != nil && ws.Status.Text != "" {
			fmt.Printf("  Status: %s%s\n", ws.Status.Text, statusMeta(ws.Status))
		}
		fmt.Println()
		fmt.Printf("  %s%s %s %s %s LAST COMMIT%s\n", b,
			render.PadRight("REPO", repoW), render.PadRight("BRANCH", branchW),
			render.PadRight("STATUS", statusW), render.PadRight("AHEAD/BEHIND", abW), r)
		fmt.Printf("  %s %s %s %s %s\n",
			render.PadRight(strings.Repeat("─", repoW-1), repoW),
			render.PadRight(strings.Repeat("─", branchW-1), branchW),
			render.PadRight(strings.Repeat("─", 8), statusW),
			render.PadRight(strings.Repeat("─", 12), abW),
			strings.Repeat("─", 11))

		for _, rp := range ws.Repos {
			if rp.Status == "missing" {
				fmt.Printf("  %s %s\n", render.PadRight(rp.Name, repoW), render.PadRight("(missing)", branchW))
				continue
			}
			a, bv := "?", "?"
			if rp.Ahead != nil {
				a = strconv.Itoa(*rp.Ahead)
			}
			if rp.Behind != nil {
				bv = strconv.Itoa(*rp.Behind)
			}
			abPlain := fmt.Sprintf("+%s / -%s", a, bv)
			abPadded := render.PadRight(abPlain, abW)
			commitRel := ""
			if rp.LastCommit != nil {
				commitRel = render.RelativeTime(*rp.LastCommit)
			}
			fmt.Printf("  %s %s %s %s %s\n",
				render.PadRight(rp.Name, repoW), render.PadRight(rp.Branch, branchW),
				render.PadRight(rp.Status, statusW), abPadded, commitRel)
		}
	}
	fmt.Println()
}

func runStatusSet(args []string) {
	p := loadPlatform()
	by := os.Getenv("USER")
	if by == "" {
		by = "unknown"
	}
	text := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--by" {
			if i+1 < len(args) {
				by = args[i+1]
				i++
			}
			continue
		}
		text = args[i]
	}
	if text == "" {
		render.Die("Usage: becket status set <text> [--by <who>]")
	}
	id := requireWorkspaceID("", "Usage: becket status set <text> [--by <who>]")
	m := mustLoadManifest(p, id)
	m.Status = &workspace.Status{
		Text:      text,
		UpdatedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedBy: by,
	}
	mustSaveManifest(p, id, m)
	render.Info("Status set for %s: %s", id, text)
}

func runStatusClear(args []string) {
	p := loadPlatform()
	id := requireWorkspaceID(arg0(args), "Usage: becket status clear [id]")
	m := mustLoadManifest(p, id)
	m.Status = nil
	mustSaveManifest(p, id, m)
	render.Info("Status cleared for %s", id)
}

func parseCount(s string) *int {
	if s == "?" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

func mustLoadManifest(p *config.Platform, id string) *workspace.Manifest {
	m, err := workspace.Load(manifestPath(p, id))
	if err != nil {
		render.Die("Workspace '%s' not found.", id)
	}
	return m
}

func mustSaveManifest(p *config.Platform, id string, m *workspace.Manifest) {
	if err := workspace.Save(manifestPath(p, id), m); err != nil {
		render.Die("%v", err)
	}
}
