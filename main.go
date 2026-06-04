// Command becket is a cross-repo git-worktree CLI. This is the Go + Cobra
// reimplementation of the original bash script (bin/becket); see
// tests/README.md for the characterization suite that pins behavioural parity.
package main

import (
	"embed"

	"github.com/Mykhol/becket/internal/cli"
)

// version is injected at build time via -ldflags "-X main.version=<v>".
// It defaults to "dev" for `go run` / un-stamped builds.
var version = "dev"

// schemaFS carries the JSON schemas becket writes alongside config. Embedding
// them removes the install-time share/ directory the bash script depended on.
//
//go:embed schema/settings.schema.json schema/workspace.schema.json
var schemaFS embed.FS

func main() {
	cli.Execute(version, schemaFS)
}
