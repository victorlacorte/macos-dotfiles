# My macOS dotfiles

## Requirements

```sh
stow -n -v --target=$HOME ...
brew install pinentry-mac
brew install tree-sitter-cli
brew install fzf jq
```

## Tmux

`prefix Space` opens the project sessionizer.

`prefix u` opens a Claude agent picker. It lists running Claude Code agents,
shows a live tmux pane preview, jumps to the selected agent with `enter`, and
kills the selected Claude process with `ctrl-x`.

The Claude picker scripts are adapted from `craftzdog/tmux-claude-session-manager`;
see `THIRD_PARTY_NOTICES.md` for the upstream MIT license notice.

The picker requires tmux 3.2 or newer, `fzf`, `jq`, and a Claude Code version
that supports `claude agents --json`.

Codex follow-up: Codex does not have a direct `claude agents --json` equivalent
for live local agents. A future Codex picker should use Codex app-server or SDK
metadata, especially `thread/list`, `thread/loaded/list`, and thread status
events. A simpler interim version could list dedicated `codex-*` tmux sessions
with previews, but it would not have reliable working/waiting/idle status.

## Git

Run `gpg --list-secret-keys --keyid-format LONG`, pick the `[S]` key and add the full subkey fingerprint for a `.gitconfig.local` file in `./git/`:

```sh
[user]
signingkey = 123abc...!
```

The following block allows for a simple end-to-end test:

```sh
tmp=$(mktemp -d)
git -C "$tmp" init
git -C "$tmp" commit --allow-empty -S -m "test signing"
git -C "$tmp" log --show-signature -1
```

If it works, `git log --show-signature -1` should show a good GPG signature using the configured key.

## Neovim config

- Easily validate new configs:

```sh
XDG_CONFIG_HOME=./nvim-0.11/ nvim nvim-0.11/nvim/init.lua
```
