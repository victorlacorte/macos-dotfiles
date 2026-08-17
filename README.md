# My macOS dotfiles

## Requirements

```sh
stow --no --verbose --target=$HOME ...
brew install pinentry-mac
brew install tree-sitter-cli
brew install fzf
```

The tmux agent picker and snapshot sources live in `tools/agent-picker` and
`tools/tmux-snapshot`, and require Go 1.22 or newer. Build and install them
locally from the repository root with:

```sh
make install-agent-picker
make install-tmux-snapshot
```

These install `agent-picker` and `tmux-snapshot` in `$HOME/.local/bin`.
Re-run the relevant command after updating these dotfiles to rebuild the
binary. The binaries are local build artifacts and are not committed.

## Codex

The `agents/.codex/general.config.toml` profile grants Codex's
`workspace-write` sandbox access to `~/.codex/plans/`, which is required for
persisting Plan Mode handoffs. Stow it to create
`~/.codex/general.config.toml`:

```sh
mkdir -p "$HOME/.codex/plans"
stow --target="$HOME" agents
```

The existing private `~/.codex/config.toml` remains untouched. The `codex`
function in `zsh/.zshrc` automatically supplies `--profile general` unless an
explicit profile is provided; use `command codex ...` to bypass the wrapper.
`AGENTS.md` documents this permission but cannot grant it.

## Tmux

`prefix Space` opens the project sessionizer.

`prefix u` opens one picker for running Claude Code, Codex, and Cursor TUI
panes. It shows each provider, activity age, tmux location, working
path, and a live pane preview. Press `enter` to jump to an agent or `ctrl-x` to
terminate its process and reload the list.

Claude is discovered from interactive session files under
`CLAUDE_CONFIG_DIR` (or `~/.claude`) and joined to tmux panes through `ps` and
TTYs. Codex is detected passively by joining exact `codex` processes to tmux
panes through their TTYs. Its age comes from the newest open
`$CODEX_HOME/sessions/**/rollout-*.jsonl` when `lsof` is available.

Cursor is detected passively as well.
Its launcher replaces `argv[0]` with the invoked path, usually a link such as
`~/.local/bin/agent`, so a process belongs to Cursor when that path resolves
into the Cursor installation root. The same test covers the bundled `node`
processes a session spawns, and among the matching processes sharing a TTY the
picker reports the topmost one, which is the session leader `ctrl-x` must
terminate. Its age comes from the newest open `store.db*` under the Cursor
configuration directory's `chats` directory when `lsof` is available. The
configuration directory is resolved from `CURSOR_CONFIG_DIR`, then
`$XDG_CONFIG_HOME/cursor`, then `~/.cursor`.

Only tmux 3.2 or newer and `fzf` are required at runtime. Codex enables the
Codex provider; the launcher named by `@cursor_agent_process_name` enables the
Cursor provider (default: `cursor-agent`); and `lsof` adds Codex and Cursor
activity ages. Claude session files are decoded directly, so neither the Claude
executable nor `jq` is required on `PATH`. A missing optional command disables
only that provider or metadata.

The popup and `fzf` frame appear immediately while agent discovery runs. When
no running agents match the selected provider, the empty picker closes and a
concise tmux message appears on the originating client. `agent-picker list`
remains machine-readable, producing no output and exiting successfully for an
empty result.

The command requires an explicit action and accepts an optional provider:

```text
agent-picker popup [-provider all|claude|codex|cursor] [TMUX_CLIENT]
agent-picker select [-provider all|claude|codex|cursor]
agent-picker list [-provider all|claude|codex|cursor]
```

Both `-provider` and `--provider` are accepted.
For `popup`, place the provider flag before `TMUX_CLIENT`.

The generic tmux options and defaults are:

```tmux
set -g @agent_popup_width       '90%'
set -g @agent_popup_height      '90%'
set -g @agent_fzf_options       ''
set -g @codex_agent_process_name 'codex'
set -g @cursor_agent_process_name 'cursor-agent'
```

`@cursor_agent_process_name` is the launcher the picker looks up on `PATH` and
accepts as a Cursor process name. It defaults to `cursor-agent`; `agent`,
`cursor-agent`, and the configured launcher's base name are recognized. A
process using one of those names is still only reported once its path resolves
into the Cursor installation root.

Selecting an agent focuses its tmux session, window, and pane in place.

The provider adapters use structured Go values internally and only format TSV
at the fzf boundary, so a future Codex app-server adapter can replace passive
process discovery and add richer activity metadata without changing the picker
UI.

The Claude metadata adapter is adapted from
`craftzdog/tmux-claude-session-manager`; see `THIRD_PARTY_NOTICES.md` for the
upstream MIT license notice.

### Tmux snapshots

Save and restore tmux sessions, windows, working directories, window indices,
manual window names, and the active session/window:

```sh
tmux-snapshot save
tmux-snapshot restore
```

Snapshots are stored in
`${XDG_STATE_HOME:-$HOME/.local/state}/tmux-snapshot/`. The `latest` symlink is
used by restore when no file is provided; if it is missing, restore falls back to
the newest `.json` file in that directory. Snapshots are JSON files named with
their UTC save time. Running processes, scrollback, pane splits, and pane layouts
are not captured. Pass a file path to either command to save or restore a
specific snapshot. There is no picker: selecting an older snapshot means naming
its path.

Saving to an explicit path does not update the `latest` symlink, so such a
snapshot never becomes the default for restore.

Restore only adds sessions. A session whose name already exists is left
untouched, and one whose recorded path no longer exists is skipped with a
warning. A session that fails partway through is rolled back and killed, leaving
the rest of the snapshot to restore normally.

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
