# Tmux

Install the tools first (see the README): `make install-agent-picker
install-tmux-snapshot`. Runtime needs tmux 3.2 or newer and `fzf`.

| Key            | Action                                          |
| -------------- | ----------------------------------------------- |
| `prefix Space` | Project sessionizer (`scripts/tmux-sessionizer`) |
| `prefix u`     | Agent picker                                    |

The prefix is `C-a`.

## Agent picker

One picker for foreground Claude Code, Codex, and Cursor CLI sessions in tmux
panes. It shows provider, approximate last-write age, tmux location, working
path, and a live pane preview. `enter` jumps to the session; `ctrl-x` sends
SIGTERM to its process and reloads the list. When nothing matches, the picker
closes and shows a tmux message.

```text
agent-picker popup [-provider all|claude|codex|cursor] [TMUX_CLIENT]
agent-picker select [-provider all|claude|codex|cursor]
agent-picker list [-provider all|claude|codex|cursor] [-identities]
agent-picker terminate -provider PROVIDER -pane PANE -pid PID -identity IDENTITY
```

Place the provider flag before `TMUX_CLIENT`. `list` prints seven TSV fields
(and nothing, with exit 0, when empty); `-identities` appends an opaque eighth
field that `terminate` requires.

Optional tmux options and defaults:

```tmux
set -g @agent_popup_width       '90%'
set -g @agent_popup_height      '90%'
set -g @agent_fzf_options       ''
set -g @codex_agent_process_name 'codex'
set -g @cursor_agent_process_name '' # automatic cursor-agent / agent discovery
```

`@cursor_agent_process_name` may be `agent`, `cursor-agent`, or an absolute
path. An explicitly configured missing launcher disables the Cursor provider.

### How discovery works

Passive and best-effort. Processes are joined to panes by TTY and must be in
the foreground process group. Headless and service invocations (Claude/Cursor
print mode, `codex exec`, `review`, `app-server`) are excluded, as are
detached, IDE, desktop, and cloud sessions. Processes with unreadable
arguments or start times are omitted. Several independent matching roots in
one foreground job are ambiguous and omitted.

- Claude: session files under `CLAUDE_CONFIG_DIR` (or `~/.claude`), checked
  against the live command and process start. Age is the newest matching
  project transcript mtime. Neither `claude` nor `jq` needs to be on PATH.
- Codex: process basename from `@codex_agent_process_name`. Age comes from
  open `$CODEX_HOME/sessions/**/rollout-*.jsonl` files.
- Cursor: `cursor-agent` or `agent` resolved on PATH into the Cursor
  installation. Age comes from open `store.db` or `store.db-wal` files under
  `chats` in `CURSOR_CONFIG_DIR`, `$XDG_CONFIG_HOME/cursor`, or `~/.cursor`.

Age is an approximate write-recency signal. Without `lsof` or a matching open
file it shows `-`.

### Termination safety

Before SIGTERM the picker rediscovers the provider and pane and rechecks the
process start time (second precision from `ps`), command, parent, TTY, process
group, and invocation mode. Changed or backgrounded targets are refused. Only
the validated PID is signalled, not its descendants.

The Claude metadata adapter is adapted from
`craftzdog/tmux-claude-session-manager`; see `THIRD_PARTY_NOTICES.md`.

## Snapshots

```sh
tmux-snapshot save
tmux-snapshot restore
```

A snapshot (version 2) records session names, directories, and which session
was attached, in session-index order. Windows, panes, processes, layouts, and
scrollback are not captured. Version 1 snapshots are unsupported: run `save`
again after upgrading.

It is stored as JSON at
`${XDG_STATE_HOME:-$HOME/.local/state}/tmux-snapshot/latest` and replaced
atomically on each save. Neither command takes a path. An older `latest`
symlink can still be restored, and the next save replaces it.

Restore only adds sessions. Existing names are left untouched, sessions whose
path is gone are skipped with a warning, and one failure does not stop the
rest. Each created session gets one default shell window. On an empty server
the saved order is reproduced; otherwise only the relative order among
created sessions is kept, since tmux cannot reassign session IDs.
