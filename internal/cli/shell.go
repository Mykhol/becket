package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Mykhol/becket/internal/render"
)

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell [id]",
		Short: "Print workspace directory path (use with shell-init)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			p := loadPlatform()
			id := requireWorkspaceID(arg0(args), "Usage: becket shell [id] (or run from inside a workspace)")
			path := wsPath(p, id)
			if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
				render.Die("Workspace '%s' not found.", id)
			}
			fmt.Println(path)
		},
	}
}

func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init",
		Short: "Output shell wrapper for eval (enables 'becket shell' as cd)",
		Args:  cobra.NoArgs,
		Run:   func(_ *cobra.Command, _ []string) { printShellInit() },
	}
}

// printShellInit emits a wrapper that turns `becket shell` into a cd, plus
// loads Cobra-generated completions for the current shell.
func printShellInit() {
	fmt.Print(`becket() {
  if [[ "$1" == "shell" ]]; then
    local _becket_dir
    _becket_dir="$(command becket shell "${@:2}")" || return $?
    builtin cd "$_becket_dir"
  else
    command becket "$@"
  fi
}
if [[ -n "$ZSH_VERSION" ]]; then
  source <(command becket completion zsh)
elif [[ -n "$BASH_VERSION" ]]; then
  source <(command becket completion bash)
fi
`)
}

func arg0(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}
