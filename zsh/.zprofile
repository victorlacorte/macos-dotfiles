eval "$(/opt/homebrew/bin/brew shellenv)"

# Setup SSH authentication
unset SSH_AGENT_PID

if [ "${gnupg_SSH_AUTH_SOCK_by:-0}" -ne $$ ]; then
  export SSH_AUTH_SOCK="$(gpgconf --list-dirs agent-ssh-socket)"
fi

export GPG_TTY=$(tty)

SSH_KEYGRIP_FILE="$HOME/.gnupg/ssh-keygrip"
if [ ! -f "$SSH_KEYGRIP_FILE" ]; then
  echo "Error: $SSH_KEYGRIP_FILE not found. Create it with GPG keygrip." >&2
  return 1
fi

gpg-connect-agent "keyattr $(cat "$SSH_KEYGRIP_FILE") Use-for-ssh: true" /bye >/dev/null

# Path: https://zsh.sourceforge.io/Guide/zshguide02.html, section 2.5.11
# Zsh-specific: keep only unique entries on path
typeset -U path
path=(/usr/local/go/bin $HOME/coding/macos-dotfiles/scripts $path)

# Set a proper locale
export LANG="en_US.UTF-8"
