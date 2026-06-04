// Package cli wires up the Cobra command tree for becket.
package cli

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/render"
)

// schemas holds the embedded JSON schemas, injected from main via Execute so
// commands (init, upgrade) can write them alongside config.
var schemas embed.FS

// appVersion is the build-stamped version string (e.g. "0.1.0").
var appVersion string

// Execute builds the command tree and runs it. version and schemaFS are passed
// from package main (the latter embedded at the module root).
func Execute(version string, schemaFS embed.FS) {
	appVersion = version
	schemas = schemaFS

	// Usage logging fires for every invocation before dispatch, matching the
	// bash main() (cmd defaults to "help" when no args are given).
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	logUsage(cmd)

	var showVersion bool
	root := &cobra.Command{
		Use:           "becket",
		Short:         "Cross-repo worktree CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(_ *cobra.Command, args []string) {
			switch {
			case showVersion:
				printVersion()
			case len(args) > 0:
				render.Die("Unknown command: %s. Run 'becket help' for usage.", args[0])
			default:
				printHelp()
			}
		},
	}
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version")
	root.SetHelpFunc(func(_ *cobra.Command, _ []string) { printHelp() })

	root.AddCommand(
		newInitCmd(),
		newVersionCmd(),
		newListCmd(),
		newStatusCmd(),
		newDescCmd(),
		newCreateCmd(),
		newAdoptCmd(),
		newAddCmd(),
		newTeardownCmd(),
		newSetupCmd(),
		newShellCmd(),
		newShellInitCmd(),
		newSyncCmd(),
		newRestackCmd(),
		newPushCmd(),
		newPRCmd(),
		newLogCmd(),
		newStatsCmd(),
		newUpgradeCmd(),
		newDevCmd(),
	)

	if err := root.Execute(); err != nil {
		// Reformat Cobra's unknown-command error to match the bash dispatch.
		const prefix = `unknown command "`
		if msg := err.Error(); strings.HasPrefix(msg, prefix) {
			name := msg[len(prefix):]
			if i := strings.IndexByte(name, '"'); i >= 0 {
				name = name[:i]
			}
			render.Die("Unknown command: %s. Run 'becket help' for usage.", name)
		}
		render.Die("%v", err)
	}
}

func printVersion() {
	fmt.Printf("becket v%s\n", appVersion)
}
