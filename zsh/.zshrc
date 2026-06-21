unsetopt BEEP

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
