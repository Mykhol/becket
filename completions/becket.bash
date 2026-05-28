# Bash completion for becket — cross-repo worktree CLI

__becket_workspaces_dir() {
  local dir="$PWD"
  while [[ "$dir" != "/" ]]; do
    if [[ -f "$dir/.becket/settings.json" ]]; then
      echo "$dir/.becket/workspaces"
      return
    fi
    dir="$(dirname "$dir")"
  done
}

__becket_config_file() {
  local dir="$PWD"
  while [[ "$dir" != "/" ]]; do
    if [[ -f "$dir/.becket/settings.json" ]]; then
      echo "$dir/.becket/settings.json"
      return
    fi
    dir="$(dirname "$dir")"
  done
}

__becket_workspace_ids() {
  local ws_dir
  ws_dir="$(__becket_workspaces_dir)" || return
  [[ -d "$ws_dir" ]] || return
  for d in "$ws_dir"/*/; do
    [[ -f "$d/.becket.json" ]] && basename "$d"
  done
}

__becket_repo_names() {
  local config
  config="$(__becket_config_file)" || return
  python3 -c "
import json, sys
d = json.load(open(sys.argv[1]))
for k in d.get('repos', {}):
    print(k)
" "$config" 2>/dev/null
}

_becket() {
  local cur prev cmd
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  # First argument: command
  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "init create list status teardown add setup dev shell shell-init sync restack push pr log stats upgrade help version" -- "$cur") )
    return
  fi

  cmd="${COMP_WORDS[1]}"
  case "$cmd" in
    create)
      case "$prev" in
        --desc|--base)  return ;;
        --repos)        COMPREPLY=( $(compgen -W "$(__becket_repo_names)" -- "$cur") ); return ;;
        --stacked-on)   COMPREPLY=( $(compgen -W "$(__becket_workspace_ids)" -- "$cur") ); return ;;
      esac
      COMPREPLY=( $(compgen -W "--desc --repos --base --stacked-on --setup" -- "$cur") )
      ;;
    status|setup|dev|shell|sync|restack|push|pr|log)
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$(__becket_workspace_ids)" -- "$cur") )
      fi
      ;;
    teardown)
      case "$prev" in
        teardown)
          COMPREPLY=( $(compgen -W "$(__becket_workspace_ids)" -- "$cur") )
          ;;
        *)
          COMPREPLY=( $(compgen -W "--delete-branches $(__becket_workspace_ids)" -- "$cur") )
          ;;
      esac
      ;;
    add)
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$(__becket_workspace_ids)" -- "$cur") )
      elif [[ $COMP_CWORD -eq 3 ]]; then
        COMPREPLY=( $(compgen -W "$(__becket_repo_names)" -- "$cur") )
      fi
      ;;
  esac
}

complete -F _becket becket
