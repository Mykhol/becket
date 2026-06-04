package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
)

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log [id]",
		Short: "Show commits across all repos since branching",
		Args:  cobra.MaximumNArgs(1),
		Run:   func(_ *cobra.Command, args []string) { runLog(args) },
	}
}

func runLog(args []string) {
	p := loadPlatform()
	id := requireWorkspaceID(arg0(args), "Usage: becket log [id] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)

	b, dim, r := "", "", ""
	if render.IsTTY() {
		b, dim, r = "\033[1m", "\033[2m", "\033[0m"
	}

	ws := wsPath(p, id)
	for _, repo := range m.Order {
		wt := filepath.Join(ws, repo)
		branch := m.Repos[repo].Branch
		base := m.Repos[repo].Base
		fmt.Println()
		fmt.Printf("%s%s%s  %s%s%s\n", b, repo, r, dim, branch, r)
		fmt.Println(strings.Repeat("─", 50))
		if git.RunStdout(wt, "log", "--oneline", "--graph", "origin/"+base+".."+branch) != nil {
			gitOrExit(git.Run(wt, "log", "--oneline", "-10"))
		}
	}
	fmt.Println()
}
