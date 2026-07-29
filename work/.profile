eval "$(/opt/homebrew/bin/brew shellenv)"

path_prepend() {
  local require_directory=
  local directory
  local remaining=${PATH:-}
  local entry
  local updated_path=
  local entry_count=0
  local last_entry=

  if [ "$1" = "-d" ]; then
    require_directory=1
    shift
  fi

  directory=$1

  if [ -n "$require_directory" ] && [ ! -d "$directory" ]; then
    return 0
  fi

  while [ -z "$last_entry" ]; do
    case "$remaining" in
    *:*)
      entry=${remaining%%:*}
      remaining=${remaining#*:}
      ;;
    *)
      entry=$remaining
      last_entry=1
      ;;
    esac

    if [ "$entry" != "$directory" ]; then
      if [ "$entry_count" -eq 0 ]; then
        updated_path=$entry
      else
        updated_path="$updated_path:$entry"
      fi
      entry_count=$((entry_count + 1))
    fi
  done

  if [ "$entry_count" -eq 0 ]; then
    PATH=$directory
  else
    PATH="$directory:$updated_path"
  fi
}

# shellenv prepends Homebrew paths without removing inherited occurrences.
path_prepend "$HOMEBREW_PREFIX/sbin"
path_prepend "$HOMEBREW_PREFIX/bin"

# VOLTA_HOME="$HOME/.volta"
# PNPM_HOME="$HOME/Library/pnpm"
LDFLAGS="-L/opt/homebrew/opt/openssl@3/lib"
CPPFLAGS="-I/opt/homebrew/opt/openssl@3/include"

path_prepend -d /opt/homebrew/opt/libpq/bin
path_prepend -d "$HOME/.bun/bin"
path_prepend -d "$HOME/.cargo/bin"
path_prepend -d "$HOME/.poetry/bin"
path_prepend -d "$HOME/bin"
path_prepend -d "$HOME/go/bin"
path_prepend -d "$HOME/.local/bin"
# path_prepend -d "$HOME/coding/victorlacorte/scripts/bin"
path_prepend -d "$HOME/coding/victorlacorte/macos-dotfiles/scripts"
# path_prepend -d "$VOLTA_HOME/bin"
path_prepend -d '/Applications/Visual Studio Code.app/Contents/Resources/app/bin'
path_prepend -d /Library/Frameworks/Python.framework/Versions/3.10/bin
path_prepend -d /Users/victor/Library/Python/3.10/bin
path_prepend -d /usr/local/texlive/2023/bin/universal-darwin

# Either this is necessary or 'ls -G'
export CLICOLOR=1 # the 'export' is required here

# Set gpg-agent for SSH authentication
unset SSH_AGENT_PID

if [ "${gnupg_SSH_AUTH_SOCK_by:-0}" -ne $$ ]; then
  SSH_AUTH_SOCK="$(gpgconf --list-dirs agent-ssh-socket)"
fi

GPG_TTY=$(tty)
# gpg-connect-agent updatestartuptty /bye >/dev/null

SSH_KEYGRIP_FILE="$HOME/.gnupg/ssh-keygrip"
if [ ! -f "$SSH_KEYGRIP_FILE" ]; then
  echo "Error: $SSH_KEYGRIP_FILE not found. Create it with GPG keygrip." >&2
  return 1
fi

gpg-connect-agent "keyattr $(cat "$SSH_KEYGRIP_FILE") Use-for-ssh: true" /bye >/dev/null

# TODO almost sure zsh does not load the .profile like bash does. Instead,
# there is .zprofile
# if [ -n "$ZSH_VERSION" ] && [ -f "$HOME/.zshrc" ]; then
# 	. "$HOME/.zshrc"
# fi

# TODO Currently, it is not possible to install older Node versions with PNPM since
# it will attempt to resolve an incorrect URL. For example:
#
# https://nodejs.org/download/release/v14.17.1/node-v14.17.1-darwin-arm64.tar.gz
#
# while the correct one would be:
#
# https://nodejs.org/dist/v14.17.1/node-v14.17.1-darwin-x64.tar.gz
#
# if [ -d "$PNPM_HOME" ]; then
#     PATH="$PNPM_HOME:$PATH"
# fi

if [ -n "$BASH_VERSION" ] && [ -f "$HOME/.bashrc" ]; then
  . "$HOME/.bashrc"
fi
