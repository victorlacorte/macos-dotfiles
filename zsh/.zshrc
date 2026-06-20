unsetopt BEEP

# Enable autocompletions
# Git
# TODO this is not working
autoload -Uz compinit && compinit
# Pnpm
# TODO review if it makes sense to maintain this completion here or after activating mise
#source "$HOME/completion-for-pnpm.zsh"

# Oh My Posh
# if [ "$TERM_PROGRAM" != "Apple_Terminal" ]; then
#   eval "$(oh-my-posh init zsh --config $HOME/.config/oh-my-posh/themes/takuya-modified.json)"
# fi

eval "$(~/.local/bin/mise activate zsh)"
eval "$(starship init zsh)"

# fzf
export FZF_CTRL_R_OPTS="--height 40% --reverse --border --tiebreak=index"
source <(fzf --zsh)

# Aliases
alias ls='ls --color=auto'
alias ll='ls -lhAF'
alias vim='nvim'
alias pn='pnpm'
alias grep='grep --color=always'
