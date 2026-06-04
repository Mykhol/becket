package cli

import (
	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/render"
)

func newDescCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "desc [id] <text>",
		Short: "Set workspace description",
		Long: `Set a workspace's description. The id is detected from the current
directory when only the text is given.`,
		DisableFlagParsing: true, // description text may contain leading dashes
		Run: func(cmd *cobra.Command, args []string) {
			if helpRequested(cmd, args) {
				return
			}
			runDesc(args)
		},
	}
}

// runDesc mirrors cmd_desc's flexible parsing: `desc <text>` (id from CWD) or
// `desc <id> <text>`.
func runDesc(args []string) {
	p := loadPlatform()
	text, wsID := "", ""
	for _, a := range args {
		if text == "" {
			text = a
		} else {
			wsID = text
			text = a
		}
	}
	if text == "" {
		render.Die("Usage: becket desc [id] <text>")
	}
	if wsID == "" {
		wsID = requireWorkspaceID("", "Usage: becket desc [id] <text>")
	}
	m := mustLoadManifest(p, wsID)
	m.Description = text
	mustSaveManifest(p, wsID, m)
	render.Info("Description set for %s: %s", wsID, text)
}
