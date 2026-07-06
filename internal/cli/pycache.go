package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mykhol/becket/internal/git"
	"github.com/Mykhol/becket/internal/render"
)

// removePycacheOrphans deletes directories that a rebase left holding nothing
// but Python bytecode caches. When upstream deletes a package, its gitignored
// `__pycache__/` survives the rebase and keeps the directory alive — an empty
// package dir on sys.path shadows same-named modules, so imports silently
// resolve to the deleted package.
//
// git collapses an all-ignored/untracked directory to a single trailing-slash
// entry, and only at the topmost level with no tracked files — so an entry
// like `pkg/logger/` is exactly an orphan candidate, while a live package's
// cache surfaces as `pkg/live/__pycache__/` and is skipped.
func removePycacheOrphans(wt string) {
	out, err := git.Output(wt, "status", "--porcelain", "--ignored")
	if err != nil || out == "" {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 || !strings.HasSuffix(line, "/") {
			continue
		}
		st, rel := line[:2], line[3:]
		if st != "!!" && st != "??" {
			continue
		}
		if filepath.Base(filepath.Clean(rel)) == "__pycache__" {
			continue
		}
		abs := filepath.Join(wt, rel)
		if !holdsOnlyPycache(abs) {
			continue
		}
		if os.RemoveAll(abs) == nil {
			render.Info("Removed orphaned Python cache: %s", rel)
		}
	}
}

// holdsOnlyPycache reports whether dir's recursive contents are solely
// compiled-Python artifacts (.pyc/.pyo files, in __pycache__ dirs or not).
func holdsOnlyPycache(dir string) bool {
	only := true
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			only = false
			return fs.SkipAll
		}
		if d.IsDir() || strings.HasSuffix(d.Name(), ".pyc") || strings.HasSuffix(d.Name(), ".pyo") {
			return nil
		}
		only = false
		return fs.SkipAll
	})
	return only
}
