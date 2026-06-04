package cli

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/jsonfmt"
	"github.com/Mykhol/becket/internal/render"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .becket/ in current directory",
		Args:  cobra.NoArgs,
		Run:   func(_ *cobra.Command, _ []string) { runInit() },
	}
}

// runInit ports cmd_init: scan the current directory for sibling git repos,
// detect each repo's default base (main vs master), and write settings.json.
func runInit() {
	cwd, err := os.Getwd()
	if err != nil {
		render.Die("%v", err)
	}

	becketDir := filepath.Join(cwd, ".becket")
	configFile := filepath.Join(becketDir, "settings.json")
	if _, err := os.Stat(configFile); err == nil {
		render.Die(".becket/settings.json already exists in this directory.")
	}

	render.Info("Scanning for git repos in %s...", cwd)

	entries, err := os.ReadDir(cwd)
	if err != nil {
		render.Die("%v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // deterministic, locale-independent ordering

	repos := map[string]config.RepoConfig{}
	for _, name := range names {
		gitDir := filepath.Join(cwd, name, ".git")
		if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
			continue
		}
		repoPath := filepath.Join(cwd, name)
		base := "main"
		if git.Verify(repoPath, "origin/master") && !git.Verify(repoPath, "origin/main") {
			base = "master"
		}
		repos[name] = config.RepoConfig{Path: "./" + name, DefaultBase: base}
		render.Info("Found repo: %s (base: %s)", name, base)
	}

	if len(repos) == 0 {
		render.Die("No git repos found in current directory.")
	}

	if err := os.MkdirAll(becketDir, 0o755); err != nil {
		render.Die("%v", err)
	}

	// Write the embedded schema alongside config and reference it.
	schemaRef := ""
	if data, err := schemas.ReadFile("schema/settings.schema.json"); err == nil {
		if err := os.WriteFile(filepath.Join(becketDir, "settings.schema.json"), data, 0o644); err == nil {
			schemaRef = "./settings.schema.json"
		}
	}

	settings := config.Settings{
		Schema:       schemaRef,
		Repos:        repos,
		BranchPrefix: "feature/",
	}
	data, err := jsonfmt.Indent2(settings)
	if err != nil {
		render.Die("%v", err)
	}
	if err := os.WriteFile(configFile, data, 0o644); err != nil {
		render.Die("%v", err)
	}

	render.Info("Created %s", configFile)
}
