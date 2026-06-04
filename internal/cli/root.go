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

// command groups for the help listing (traditional CLI layout).
var commandGroups = []*cobra.Group{
	{ID: "platform", Title: "Platform Commands:"},
	{ID: "workspace", Title: "Workspace Commands:"},
	{ID: "develop", Title: "Develop Commands:"},
	{ID: "ship", Title: "Sync & Ship Commands:"},
}

// groupOf maps each command name to its help group.
var groupOf = map[string]string{
	"init": "platform", "list": "platform", "stats": "platform", "upgrade": "platform",
	"create": "workspace", "adopt": "workspace", "add": "workspace", "teardown": "workspace",
	"shell": "develop", "shell-init": "develop", "dev": "develop", "setup": "develop",
	"status": "ship", "desc": "ship", "log": "ship", "sync": "ship",
	"restack": "ship", "push": "ship", "pr": "ship",
}

// Execute builds the command tree and runs it. version and schemaFS are passed
// from package main (the latter embedded at the module root).
func Execute(version string, schemaFS embed.FS) {
	appVersion = version
	if appVersion == "" { // never print "becket v" with no version
		appVersion = "dev"
	}
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
		Use:   "becket",
		Short: "Cross-repo git-worktree workspaces",
		Long: "becket — cross-repo git-worktree workspaces.\n\n" +
			"Create feature-scoped workspaces of coordinated git worktrees across\n" +
			"multiple repositories, all on a shared feature branch.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(c *cobra.Command, _ []string) {
			if showVersion {
				printVersion()
				return
			}
			_ = c.Help()
		},
	}
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version")
	root.AddGroup(commandGroups...)

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
	for _, c := range root.Commands() {
		if g, ok := groupOf[c.Name()]; ok {
			c.GroupID = g
		}
	}

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

// helpRequested returns true if args contain -h/--help; used by commands that
// parse their own flags (DisableFlagParsing) to still honor --help.
func helpRequested(cmd *cobra.Command, args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			_ = cmd.Help()
			return true
		}
	}
	return false
}
