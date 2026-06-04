package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

// loadPlatform loads the platform config or dies with the bash guidance message.
// It also emits the "config may be outdated" warning when $schema is absent,
// matching load_config.
func loadPlatform() *config.Platform {
	p, err := config.Load()
	if err != nil {
		render.Die("No .becket/settings.json found. Run 'becket init' in your platform directory.")
	}
	if !p.HasSchema {
		render.Warn("Config may be outdated. Run becket upgrade to update.")
	}
	return p
}

// requireWorkspaceID returns the id (or detects it from CWD), dying with usage
// when neither is available.
func requireWorkspaceID(arg, usage string) string {
	id, err := workspace.Require(arg)
	if err != nil {
		render.Die("%s", usage)
	}
	return id
}

// wsPath is the workspace directory for id.
func wsPath(p *config.Platform, id string) string {
	return filepath.Join(p.WorkspacesDir, id)
}

// manifestPath is the .becket.json path for id.
func manifestPath(p *config.Platform, id string) string {
	return filepath.Join(wsPath(p, id), ".becket.json")
}

// repoAbsPath resolves a configured repo's working directory (PLATFORM_DIR +
// its relative path).
func repoAbsPath(p *config.Platform, relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(p.Dir, relPath)
}

// today returns the current date as YYYY-MM-DD (matching `date +%Y-%m-%d`).
func today() string { return time.Now().Format("2006-01-02") }

// gitOrExit exits non-zero (silently — git already printed) when a streamed git
// command fails, mirroring bash `set -e`.
func gitOrExit(err error) {
	if err != nil {
		os.Exit(1)
	}
}

// slugify lowercases, turns spaces into hyphens, and strips anything outside
// [a-z0-9-], matching the bash `tr` pipeline used for description slugs.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// selectRepos resolves which repos to include: an explicit --repos list, an
// interactive picker on a TTY, or all configured repos otherwise (the non-TTY
// default), matching create/adopt.
func selectRepos(p *config.Platform, reposFlag string) []string {
	if reposFlag != "" {
		return strings.Split(reposFlag, ",")
	}
	if isStdinTTY() {
		fmt.Println()
		fmt.Println("Select repos to include:")
		for i, name := range p.RepoOrder {
			fmt.Printf("  %d. %s\n", i+1, name)
		}
		fmt.Println("  Enter numbers separated by spaces, or press Enter for all")
		fmt.Print("> ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return append([]string(nil), p.RepoOrder...)
		}
		var sel []string
		for _, tok := range strings.Fields(line) {
			var idx int
			if _, err := fmt.Sscanf(tok, "%d", &idx); err == nil && idx >= 1 && idx <= len(p.RepoOrder) {
				sel = append(sel, p.RepoOrder[idx-1])
			}
		}
		return sel
	}
	return append([]string(nil), p.RepoOrder...)
}

func isStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
