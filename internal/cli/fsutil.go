package cli

import (
	"io"
	"os"
	"path/filepath"

	"github.com/Mykhol/becket/internal/config"
	"github.com/Mykhol/becket/internal/render"
)

// writeWorkspaceSchema writes the embedded workspace schema into the workspace
// and returns the $schema reference to record (empty if it couldn't be written).
func writeWorkspaceSchema(wsPath string) string {
	data, err := schemas.ReadFile("schema/workspace.schema.json")
	if err != nil {
		return ""
	}
	if err := os.WriteFile(filepath.Join(wsPath, "workspace.schema.json"), data, 0o644); err != nil {
		return ""
	}
	return "./workspace.schema.json"
}

// copyPlatformFiles copies the configured `files` from the platform root into the
// workspace root, warning on any that are missing (ports the create/adopt loop,
// with the empty-list case simply doing nothing — the fix for the files[@] bug).
func copyPlatformFiles(p *config.Platform, wsPath string) {
	for _, f := range p.Settings.Files {
		src := filepath.Join(p.Dir, f)
		if _, err := os.Stat(src); err != nil {
			render.Warn("File not found, skipping: %s", f)
			continue
		}
		if err := copyPath(src, filepath.Join(wsPath, filepath.Base(f))); err != nil {
			render.Warn("File not found, skipping: %s", f)
			continue
		}
		render.Info("Copied: %s", f)
	}
}

// copyPath copies a file (or directory tree) from src to dst, preserving mode.
func copyPath(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.MkdirAll(dst, fi.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
