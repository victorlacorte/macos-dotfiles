unsetopt BEEP

# Keep zsh's built-in functions available after Homebrew upgrades.
typeset -U fpath
if [[ -d "${module_path:h}/share/zsh/functions" ]]; then
  fpath=("${module_path:h}/share/zsh/functions" $fpath)
fi

# Zsh completion system
autoload -Uz compinit && compinit

eval "$(~/.local/bin/mise activate zsh)"
eval "$(starship init zsh)"

# Pnpm
source "$HOME/.config/completion-for-pnpm.zsh"

# fzf
export FZF_CTRL_R_OPTS="--height 40% --reverse --border --tiebreak=index"
source <(fzf --zsh)

# Aliases
alias ls='ls --color=auto'
alias ll='ls -lhAF'
alias vim='nvim'
alias pn='pnpm'
alias grep='grep --color=always'

codex() {
  local arg

  for arg in "$@"; do
    case "$arg" in
      --)
        break
        ;;
      --profile|-p|--profile=*)
        command codex "$@"
        return
        ;;
    esac
  done

  command codex --profile general "$@"
}
