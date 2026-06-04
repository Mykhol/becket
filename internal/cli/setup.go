package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/render"
	"github.com/Mykhol/becket/internal/workspace"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup [id]",
		Short: "Run setup commands for workspace repos",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			p := loadPlatform()
			id := requireWorkspaceID(arg0(args), "Usage: becket setup [id] (or run from inside a workspace)")
			runSetupForWorkspace(p, id)
		},
	}
}

// runSetupForWorkspace ports cmd_setup: optionally start docker, then run each
// repo's configured setup commands (with env injected) inside its worktree.
func runSetupForWorkspace(p *config.Platform, id string) {
	ws := wsPath(p, id)
	m, err := workspace.Load(manifestPath(p, id))
	if err != nil {
		render.Die("Workspace '%s' not found.", id)
	}

	if p.Settings.Docker != "" {
		composePath := filepath.Join(ws, p.Settings.Docker)
		if fi, err := os.Stat(composePath); err == nil && !fi.IsDir() {
			render.Info("Starting Docker services...")
			runShell("", "docker compose -f "+shellQuote(composePath)+" up -d")
		} else {
			render.Warn("Docker compose file not found: %s", composePath)
		}
	}

	for _, repo := range m.Order {
		rc := p.Settings.Repos[repo]
		if len(rc.Setup) == 0 {
			render.Info("No setup commands for %s, skipping.", repo)
			continue
		}
		wt := filepath.Join(ws, repo)
		if fi, err := os.Stat(wt); err != nil || !fi.IsDir() {
			render.Warn("Worktree missing for %s, skipping.", repo)
			continue
		}
		render.Info("Setting up %s...", repo)

		envPrefix := buildEnvPrefix(rc.Env)
		for _, cmd := range rc.Setup {
			render.Info("  Running: %s", cmd)
			if err := runShell(wt, envPrefix+cmd); err != nil {
				render.Warn("  Command failed: %s", cmd)
				if isStdinTTY() {
					fmt.Print("  Continue with remaining setup? [Y/n] ")
					var answer string
					fmt.Scanln(&answer)
					if strings.HasPrefix(strings.ToLower(answer), "n") {
						render.Die("Setup aborted.")
					}
				}
			}
		}
	}

	fmt.Println()
	render.Info("Setup complete for workspace %s.", id)
}

// buildEnvPrefix renders "export K=V && … && " for the repo env (keys sorted for
// determinism). Empty when there is no env.
func buildEnvPrefix(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("export %s=%s", k, env[k]))
	}
	return strings.Join(parts, " && ") + " && "
}

// runShell runs a command line via `sh -c`, streaming output, optionally in dir.
func runShell(dir, line string) error {
	cmd := exec.Command("sh", "-c", line)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
