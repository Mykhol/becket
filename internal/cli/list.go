package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/jsonfmt"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

// listEntry is the per-workspace shape for `list` (field order matches the bash
// python dict; status has no omitempty so it renders null when absent).
type listEntry struct {
	ID          string            `json:"id"`
	Created     string            `json:"created"`
	Description string            `json:"description"`
	Repos       []string          `json:"repos"`
	Status      *workspace.Status `json:"status"`
	StackParent string            `json:"stackParent"`
}

func newListCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all active workspaces",
		Args:  cobra.NoArgs,
		Run:   func(_ *cobra.Command, _ []string) { runList(output) },
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format (table|json)")
	return cmd
}

func runList(output string) {
	p := loadPlatform()

	if fi, err := os.Stat(p.WorkspacesDir); err != nil || !fi.IsDir() {
		if output == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No workspaces found.")
		}
		return
	}

	var data []listEntry
	for _, mp := range workspace.List(p.WorkspacesDir) {
		m, err := workspace.Load(mp)
		if err != nil {
			continue
		}
		data = append(data, listEntry{
			ID: m.ID, Created: m.Created, Description: m.Description,
			Repos: m.Order, Status: m.Status, StackParent: m.StackParent,
		})
	}

	if output == "json" {
		if data == nil {
			data = []listEntry{}
		}
		b, _ := jsonfmt.Indent4(data)
		fmt.Print(string(b))
		return
	}

	renderListTable(data)
}

func renderListTable(data []listEntry) {
	if len(data) == 0 {
		fmt.Println("  No active workspaces")
		return
	}
	b, r := "", ""
	if render.IsTTY() {
		b, r = "\033[1m", "\033[0m"
	}

	idW := len("ID")
	reposW := len("REPOS")
	for _, d := range data {
		idW = max(idW, len(d.ID))
		reposW = max(reposW, len(strings.Join(d.Repos, ",")))
	}
	idW += 2
	reposW += 2
	dateW := 12

	fmt.Println()
	fmt.Printf("%s%s %s %s DESCRIPTION%s\n",
		b, render.PadRight("ID", idW), render.PadRight("REPOS", reposW), render.PadRight("CREATED", dateW), r)
	fmt.Printf("%s %s %s %s\n",
		render.PadRight(strings.Repeat("─", idW-1), idW),
		render.PadRight(strings.Repeat("─", reposW-1), reposW),
		render.PadRight(strings.Repeat("─", 10), dateW),
		strings.Repeat("─", 30))

	for _, d := range data {
		reposStr := strings.Join(d.Repos, ",")
		fmt.Printf("%s %s %s %s\n",
			render.PadRight(d.ID, idW), render.PadRight(reposStr, reposW),
			render.PadRight(d.Created, dateW), d.Description)

		if d.StackParent != "" {
			fmt.Printf("  ↑ stacked on %s\n", d.StackParent)
		}
		if d.Status != nil && d.Status.Text != "" {
			fmt.Printf("  ↳ %s%s\n", truncate(d.Status.Text, 60), statusMeta(d.Status))
		}
	}
	fmt.Println()
}

// statusMeta renders the " (by, age)" suffix shared by list/status.
func statusMeta(s *workspace.Status) string {
	var parts []string
	if s.UpdatedBy != "" {
		parts = append(parts, s.UpdatedBy)
	}
	if s.UpdatedAt != "" {
		if age := render.RelativeTime(s.UpdatedAt); age != "" {
			parts = append(parts, age)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%s)", strings.Join(parts, ", "))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}
