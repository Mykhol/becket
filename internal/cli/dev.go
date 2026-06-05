package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/render"
)

func newDevCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dev [id] [--repo NAME]",
		Short: "Run repos' dev commands (concurrently, in the foreground)",
		Long: `Run each repo's configured 'dev' command in its worktree, in the foreground.

  becket dev              run every repo's dev command at once, output prefixed
                          per repo; Ctrl-C stops them all
  becket dev --repo web   run just one repo's dev command, output passed through

Docker services are started first if a 'docker' compose file is configured.`,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if helpRequested(cmd, args) {
				return
			}
			runDev(args)
		},
	}
}

type devEntry struct {
	repo, cmd, envPrefix string
}

func runDev(args []string) {
	id, only := "", ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--repo" || a == "-r":
			only = next(args, i)
			i++
		case len(a) > 0 && a[0] == '-':
			render.Die("Unknown option: %s (Usage: becket dev [id] [--repo NAME])", a)
		default:
			id = a
		}
	}

	p := loadPlatform()
	id = requireWorkspaceID(id, "Usage: becket dev [id] [--repo NAME] (or run from inside a workspace)")
	m := mustLoadManifest(p, id)
	ws := wsPath(p, id)

	// Docker services (detached) first, if configured.
	if p.Settings.Docker != "" {
		composePath := filepath.Join(ws, p.Settings.Docker)
		if fi, err := os.Stat(composePath); err == nil && !fi.IsDir() {
			render.Info("Starting Docker services...")
			runShell("", "docker compose -f "+shellQuote(composePath)+" up -d")
		} else {
			render.Warn("Docker compose file not found: %s", composePath)
		}
	}

	// Collect the repos to run.
	var entries []devEntry
	for _, repo := range m.Order {
		if only != "" && repo != only {
			continue
		}
		rc := p.Settings.Repos[repo]
		if rc.Dev == "" {
			continue
		}
		entries = append(entries, devEntry{repo: repo, cmd: rc.Dev, envPrefix: buildEnvPrefix(rc.Env)})
	}
	if len(entries) == 0 {
		if only != "" {
			render.Die("Repo '%s' has no 'dev' command in workspace '%s' (or isn't part of it).", only, id)
		}
		render.Die("No repos in workspace '%s' have a 'dev' command configured.", id)
	}

	if only != "" {
		runDevSingle(ws, entries[0])
		return
	}
	runDevMultiplexed(ws, entries)
}

// runDevSingle runs one repo's dev command in the foreground with stdio passed
// straight through, so interactive/TTY dev servers behave normally.
func runDevSingle(ws string, e devEntry) {
	render.Info("Running %s → %s", e.repo, e.cmd)
	cmd := exec.Command("sh", "-c", e.envPrefix+e.cmd)
	cmd.Dir = filepath.Join(ws, e.repo)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// runDevMultiplexed runs all dev commands concurrently, prefixing each output
// line with the repo name, and stops them all on SIGINT/SIGTERM.
func runDevMultiplexed(ws string, entries []devEntry) {
	width := 0
	for _, e := range entries {
		if len(e.repo) > width {
			width = len(e.repo)
		}
	}

	render.Info("Starting dev for %d repos (Ctrl-C to stop):", len(entries))
	for _, e := range entries {
		render.Info("  %s → %s", e.repo, e.cmd)
	}
	fmt.Println()

	var (
		outMu sync.Mutex
		wg    sync.WaitGroup
		procs []*exec.Cmd
		pMu   sync.Mutex
	)
	emit := func(prefix, line string) {
		outMu.Lock()
		fmt.Println(prefix + line)
		outMu.Unlock()
	}

	for i, e := range entries {
		prefix := devPrefix(e.repo, width, i)
		cmd := exec.Command("sh", "-c", e.envPrefix+e.cmd)
		cmd.Dir = filepath.Join(ws, e.repo)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own group, so we can kill the tree
		pr, pw, err := os.Pipe()
		if err != nil {
			render.Die("%v", err)
		}
		cmd.Stdout, cmd.Stderr = pw, pw
		if err := cmd.Start(); err != nil {
			emit(prefix, "failed to start: "+err.Error())
			pw.Close()
			pr.Close()
			continue
		}
		pMu.Lock()
		procs = append(procs, cmd)
		pMu.Unlock()

		wg.Add(1)
		go func(e devEntry, prefix string, cmd *exec.Cmd, pr, pw *os.File) {
			defer wg.Done()
			sc := bufio.NewScanner(pr)
			sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
			for sc.Scan() {
				emit(prefix, sc.Text())
			}
			err := cmd.Wait()
			pw.Close()
			pr.Close()
			if err != nil {
				emit(prefix, render.Dim("exited: "+err.Error()))
			} else {
				emit(prefix, render.Dim("exited"))
			}
		}(e, prefix, cmd, pr, pw)
	}

	// On interrupt, terminate every process group, escalating to SIGKILL.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		render.Info("Stopping...")
		pMu.Lock()
		running := append([]*exec.Cmd(nil), procs...)
		pMu.Unlock()
		for _, c := range running {
			if c.Process != nil {
				syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
			}
		}
		time.AfterFunc(5*time.Second, func() {
			for _, c := range running {
				if c.Process != nil {
					syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
				}
			}
		})
	}()

	wg.Wait()
}

// devPrefix builds a colored, padded "repo | " line prefix.
func devPrefix(repo string, width, i int) string {
	name := fmt.Sprintf("%-*s", width, repo)
	if !render.IsTTY() {
		return name + " | "
	}
	colors := []string{"\033[36m", "\033[35m", "\033[33m", "\033[32m", "\033[34m", "\033[31m"}
	return colors[i%len(colors)] + name + "\033[0m | "
}
