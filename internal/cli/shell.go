package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// printShellInit mirrors cmd_shell_init: a wrapper function that turns
// `becket shell` into a cd, plus completion bootstrap. The install prefix is
// derived from the executable location (dir, minus a trailing /bin).
func printShellInit() {
	prefix := installPrefix()
	const wrapper = `becket() {
  if [[ "$1" == "shell" ]]; then
    local _becket_dir
    _becket_dir="$(command becket shell "${@:2}")" || return $?
    builtin cd "$_becket_dir"
  else
    command becket "$@"
  fi
}
`
	fmt.Print(wrapper)
	fmt.Printf(`if [[ -n "$ZSH_VERSION" ]]; then
  fpath=("%s/share/zsh/site-functions" $fpath)
  autoload -Uz _becket 2>/dev/null && compdef _becket becket 2>/dev/null
elif [[ -n "$BASH_VERSION" ]]; then
  [[ -f "%s/share/bash-completion/completions/becket" ]] && source "%s/share/bash-completion/completions/becket"
fi
`, prefix, prefix, prefix)
}

// installPrefix resolves $PREFIX from the running binary's location: dirname,
// with a trailing /bin removed (so <prefix>/bin/becket -> <prefix>).
func installPrefix() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	return strings.TrimSuffix(dir, "/bin")
}

func arg0(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}
