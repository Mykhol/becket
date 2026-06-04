package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/workspace"
)

// usageLogPath mirrors `${XDG_DATA_HOME:-$HOME/.local/share}/becket/usage.log`.
func usageLogPath() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(base, "becket", "usage.log")
}

// logUsage appends one JSON line per invocation (cmd + detected workspace +
// platform basename), matching the bash log_usage called from main. Best-effort:
// any error is ignored, exactly as the original `|| return`.
func logUsage(cmd string) {
	workspaceID, _ := workspace.Detect()
	platform := ""
	if p, err := config.Load(); err == nil {
		platform = filepath.Base(p.Dir)
	}

	path := usageLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	entry := struct {
		Ts        string `json:"ts"`
		Cmd       string `json:"cmd"`
		Workspace string `json:"workspace"`
		Platform  string `json:"platform"`
	}{
		Ts:        time.Now().UTC().Format("2006-01-02T15:04:05.000000") + "Z",
		Cmd:       cmd,
		Workspace: workspaceID,
		Platform:  platform,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
