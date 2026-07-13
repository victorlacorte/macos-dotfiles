# My macOS dotfiles

## Requirements

```sh
stow -n -v --target=$HOME ...
brew install pinentry-mac
brew install tree-sitter-cli
brew install fzf
```

## Tmux

`prefix Space` opens the project sessionizer.

`prefix u` opens one picker for running Claude Code and Codex TUI panes. It
shows each provider, status, activity age, tmux location, working path, and a
live pane preview. Press `enter` to jump to an agent or `ctrl-x` to terminate
its process and reload the list.

Claude reports detailed `waiting`, `idle`, and `working` states through
`claude agents --json`. Codex is detected passively by joining exact `codex`
processes to tmux panes through their TTYs, so its conservative status is always
`running`. Its age comes from the newest open
`$CODEX_HOME/sessions/**/rollout-*.jsonl` when `lsof` is available.

Only tmux 3.2 or newer and `fzf` are required for the picker. Claude Code and
`jq` enable the Claude provider; Codex enables the Codex provider; and `lsof`
adds Codex activity ages. A missing optional command disables only that provider
or metadata. `claude-agent-picker` remains available as a Claude-only wrapper.

The generic tmux options and defaults are:

```tmux
set -g @agent_popup_width       '90%'
set -g @agent_popup_height      '90%'
set -g @agent_fzf_options       ''
set -g @agent_parent            '' # managed internally for popup return
set -g @codex_agent_process_name 'codex'
set -g @codex_agent_session_prefix 'codex-'
```

Existing `@claude_agent_command`, `@claude_agent_session_prefix`,
`@claude_agent_popup_width`, `@claude_agent_popup_height`,
`@claude_agent_fzf_options`, and `@claude_agent_parent` settings remain
supported. Generic popup/fzf options fall back to the old Claude values when
unset. Sessions prefixed with either `claude-` or `codex-` retain the dedicated
popup navigation behavior; agents in other sessions are focused in place.

The provider adapters share a normalized row boundary, so a future Codex
app-server adapter can replace passive process discovery and add accurate
working/waiting/idle states without changing the picker UI.

The Claude metadata adapter is adapted from
`craftzdog/tmux-claude-session-manager`; see `THIRD_PARTY_NOTICES.md` for the
upstream MIT license notice.

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
