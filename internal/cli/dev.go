package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/render"
)

func newDevCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "dev [id] [--detach]",
		Short:              "Start dev environment (docker + tmux)",
		DisableFlagParsing: true,
		Run:                func(_ *cobra.Command, args []string) { runDev(args) },
	}
}

type devEntry struct {
	repo, cmd, envPrefix string
}

func runDev(args []string) {
	id := ""
	detach := false
	for _, a := range args {
		switch {
		case a == "-d" || a == "--detach":
			detach = true
		case len(a) > 0 && a[0] == '-':
			render.Die("Unknown option: %s (Usage: becket dev [id] [--detach])", a)
		default:
			id = a
		}
	}
	p := loadPlatform()
	id = requireWorkspaceID(id, "Usage: becket dev [id] [--detach] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)

	session := p.Settings.Session
	if session == "" {
		session = id
	}
	ws := wsPath(p, id)

	if p.Settings.Docker != "" {
		composePath := filepath.Join(ws, p.Settings.Docker)
		if fi, err := os.Stat(composePath); err == nil && !fi.IsDir() {
			render.Info("Starting Docker services...")
			runShell("", "docker compose -f "+shellQuote(composePath)+" up -d")
		} else {
			render.Warn("Docker compose file not found: %s", composePath)
		}
	}

	var entries []devEntry
	for _, repo := range m.Order {
		rc := p.Settings.Repos[repo]
		if rc.Dev == "" {
			continue
		}
		entries = append(entries, devEntry{repo: repo, cmd: rc.Dev, envPrefix: buildEnvPrefix(rc.Env)})
	}
	if len(entries) == 0 {
		render.Die("No repos in workspace '%s' have a 'dev' command configured.", id)
	}

	render.Info("Starting tmux session %s...", session)
	_ = exec.Command("tmux", "kill-session", "-t", session).Run()

	for i, e := range entries {
		wt := filepath.Join(ws, e.repo)
		full := e.envPrefix + e.cmd
		if i == 0 {
			tmux("new-session", "-d", "-s", session, "-n", e.repo, "-c", wt)
		} else {
			tmux("new-window", "-t", session, "-n", e.repo, "-c", wt)
		}
		tmux("send-keys", "-t", session+":"+e.repo, full, "Enter")
		render.Info("  %s → %s", e.repo, e.cmd)
	}
	tmux("select-window", "-t", session+":"+entries[0].repo)

	if !detach && !render.IsTTY() {
		detach = true
		render.Info("No TTY detected — leaving session detached.")
	}

	if detach {
		fmt.Println()
		render.Info("Dev session %s started (detached).", session)
		render.Info("  Attach:   tmux attach -t %s", session)
		render.Info("  Logs:     tmux capture-pane -pt %s:<repo>", session)
		render.Info("  Stop:     tmux kill-session -t %s", session)
		return
	}
	attach := exec.Command("tmux", "attach-session", "-t", session)
	attach.Stdin, attach.Stdout, attach.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = attach.Run()
}

func tmux(args ...string) { _ = exec.Command("tmux", args...).Run() }
