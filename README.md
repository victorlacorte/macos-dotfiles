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
binaries. The binaries are local build artifacts and are not committed.

The `md-view` source in `tools/md-view` also requires Go 1.22 or newer. It
renders a Markdown file to a self-contained HTML preview
and opens it in the default macOS browser. Install Pandoc through mise, then
install the CLI and its Pandoc assets:

```sh
mise install pandoc
make install-md-view
```

Use `md-view FILE.md` for a browser preview or
`md-view render FILE.md --output OUTPUT.html` for a non-opening render. The
HTML embeds CSS and local images. The stylesheet is compiled from Tailwind
Typography; edit `tools/md-view/pandoc/styles/md-view.src.css` and run
`make build-md-view-css` before committing. Mermaid diagrams use one pinned
browser runtime loaded from jsDelivr, so the browser needs network access to
render those diagrams.

For development, run the Go command from the repository with the asset
directory set explicitly:

```sh
MD_VIEW_DATA_DIR="$PWD/tools/md-view" go -C tools/md-view run ./cmd/md-view FILE.md
```

Installed binaries discover the repository-relative or installed assets
automatically.

## Codex

Shared Codex settings live in `etc/codex/config.toml` and are installed
as `/etc/codex/config.toml`. The system config adds `~/.codex/plans/` as a
writable root for `workspace-write` sessions, sets the status line, and holds
the disabled `shadcn` and `chrome_devtools` MCP servers.

The Stow command in Requirements applies to packages installed under `$HOME`.
This Codex system config is the `etc` Stow package. Run from the repository
root and target `/etc`, which requires administrator privileges:

```sh
mkdir -p "$HOME/.codex/plans"
sudo "$(command -v stow)" --target=/etc etc
```

Stow stops if an unmanaged `config.toml` already exists there. Inspect it
before removing or replacing it.

Stow creates a relative symlink at `/etc/codex/config.toml`. If the repository
moves, remove that link and stow the package again from the new checkout. Use
`codex ...` directly; no shell wrapper adds arguments.

The `agents/.codex/general.config.toml` profile remains available for explicit
`--profile general` use. Stow it to create `~/.codex/general.config.toml`:

```sh
stow --target="$HOME" agents
```

The profile retains its model, reasoning, sandbox, and project trust settings.
It overlays the private `~/.codex/config.toml`; it does not replace or manage
that file. Keep client-specific values such as project trust in the private
config. `agents/.codex/AGENTS.md` documents the handoff workflow, but the
system config grants the plans directory access.

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

Save and restore tmux sessions, project directories, and the attached-session
selection:

```sh
tmux-snapshot save
tmux-snapshot restore
```

Snapshots preserve session names, directories, and whether each session was
attached when it was saved, in tmux session-index order. Restore creates one
default shell window for each missing session. Windows, panes, commands or
processes, layouts, and scrollback are not captured.

Snapshots use version 2. Version 1 snapshots are not supported. After
upgrading, run `tmux-snapshot save` before using the default restore command;
old snapshot files remain on disk and are not rewritten.

Snapshots are stored in
`${XDG_STATE_HOME:-$HOME/.local/state}/tmux-snapshot/`. The `latest` symlink is
used by restore when no file is provided; if it is missing, restore falls back to
the newest `.json` file in that directory. Snapshots are JSON files named with
their UTC save time. Pass a file path to either command to save or restore a
specific snapshot. There is no picker: selecting an older snapshot means naming
its path.

Saving to an explicit path does not update the `latest` symlink, so such a
snapshot never becomes the default for restore.

Restore only adds sessions. A session whose name already exists is left
untouched, and one whose recorded path no longer exists is skipped with a
warning. If creating one session fails, restore reports the failure and
continues with the remaining sessions.

On an empty tmux server, restore creates sessions in the saved order. If the
server already has sessions, restore leaves them untouched and preserves the
saved relative order among sessions it creates. Tmux cannot reassign existing
session IDs, so restore cannot reproduce the full saved order around sessions
that were already present.

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

## Skills

Install `jq` and GitHub CLI with its `gh skill` commands. Put
`$HOME/.local/bin` on `PATH`, as for the other local tools in this repository.
From the repository root, install the wrapper and its catalog with:

```sh
make install-skills
```

`skills/catalog.json` is the editable source recipe. `make install-skills`
refreshes the installed copy at `$HOME/.local/share/macos-dotfiles/skills/catalog.json`
and copies `skills/overlays` next to it. The wrapper uses the checkout catalog
when run directly as `./scripts/skills`; set `SKILLS_CATALOG` to use another
JSON catalog. Run the installed `skills` command from any directory:

```sh
skills install unslop
skills install --all
skills install unslop --scope project
```

`agents/.agents/skills` holds only locally authored skills (currently
`orchestrate` and `babysit`). `.gitignore` allowlists that directory; add
a new authored skill to the allowlist. Upstream skills come only from
`skills/catalog.json`.

Installs go to `~/.agents/skills`, or the Git root's `.agents/skills` with
`--scope project`. Codex and Cursor both read that directory, so one copy
serves both. Every skill there is visible to both harnesses; to keep a skill
out of automatic use, Cursor honors `disable-model-invocation: true` in
`SKILL.md` and Codex honors `policy.allow_implicit_invocation: false` in
`agents/openai.yaml` (add the latter through an overlay). User installs
target the home directory. Project installs target the Git root's
`.agents/skills`, even when you run the command from a subdirectory.
Installs reject a directory symlink at that destination. `--all` means the
six names in the local catalog, not every skill in their upstream
repositories.

`skills install` only installs missing catalog skills and skips installed
ones. `skills update` refreshes installed ones (skipping, with a notice,
catalog skills that aren't installed for `--all`) and re-applies overlays.
Removing an overlay directory and running `skills update NAME` removes its
files.

`skills/overlays/<name>/` holds files added on top of an installed upstream
skill when upstream does not ship them, such as `unslop`'s
`agents/openai.yaml` for Codex. Never copy upstream files there. The wrapper
tracks applied overlay paths in `.skills-overlay` inside the skill directory,
re-applies them on update, and skips (with a warning) any overlay file that
upstream starts shipping.

The wrapper delegates to GitHub CLI. Installed directories retain upstream
`SKILL.md`, optional `agents/openai.yaml`, supporting files, any license files
included in those directories, and GitHub source metadata. Review changed
upstream instructions when updating. Update catalog skills with the wrapper,
not raw `gh skill update`, because GitHub CLI replaces the skill directory
and drops overlay files:

```sh
skills update --all --dry-run
skills update --all
```

The wrapper passes `--dir` for the selected scope. GitHub CLI resolves the
latest release when available, otherwise the default branch head. Review the
upstream changes. `gh skill list` shows the skills GitHub CLI finds in its
known locations.

Earlier checkouts linked upstream skills into `~/.agents/skills` through
Stow. After those skills were removed from the package, the leftover
symlinks dangle, and the wrapper refuses to install over a symlinked skill
directory. Remove the dangling links, then reinstall. The `orchestrate`
link still resolves, so it is left in place. Also remove duplicate copies
left in `~/.cursor/skills` by earlier Cursor-only installs (Cursor would
otherwise see each skill twice), for example by deleting the catalog skill
directories there.

```sh
find "$HOME/.agents/skills" -maxdepth 1 -type l ! -exec test -e {} \; -print -delete
skills install --all
```
