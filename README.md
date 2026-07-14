# My macOS dotfiles

## Requirements

```sh
stow -n -v --target=$HOME ...
brew install pinentry-mac
brew install tree-sitter-cli
brew install fzf
```

The tmux agent picker source lives in `tools/agent-picker` and requires Go 1.22
or newer. Build and install it locally from the repository root with:

```sh
make install-agent-picker
```

This installs `agent-picker` in `$HOME/.local/bin`. Re-run the command after
updating these dotfiles to rebuild the binary. The binary is a local build
artifact and is not committed.

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

Only tmux 3.2 or newer and `fzf` are required at runtime. Claude Code enables
the Claude provider; Codex enables the Codex provider; and `lsof` adds Codex
activity ages. Claude JSON is decoded directly, so `jq` is no longer required.
A missing optional command disables only that provider or metadata.

When no running agents match the selected provider, the picker shows a concise
tmux message instead of opening an empty `fzf` interface. The current client
remains attached and the popup parent state is left unchanged. `agent-picker
list` remains machine-readable, producing no output and exiting successfully
for an empty result.

The command requires an explicit action and accepts an optional provider:

```text
agent-picker popup [-provider all|claude|codex] [TMUX_CLIENT]
agent-picker select [-provider all|claude|codex]
agent-picker list [-provider all|claude|codex]
```

Both `-provider` and `--provider` are accepted.
For `popup`, place the provider flag before `TMUX_CLIENT`.

The generic tmux options and defaults are:

```tmux
set -g @agent_popup_width       '90%'
set -g @agent_popup_height      '90%'
set -g @agent_fzf_options       ''
set -g @agent_parent            '' # managed internally for popup return
set -g @codex_agent_process_name 'codex'
```

Selecting an agent focuses its tmux session, window, and pane in place.

The provider adapters use structured Go values internally and only format TSV
at the fzf boundary, so a future Codex app-server adapter can replace passive
process discovery and add accurate working/waiting/idle states without changing
the picker UI.

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
