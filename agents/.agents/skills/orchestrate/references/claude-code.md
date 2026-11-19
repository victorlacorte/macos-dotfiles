# Claude Code adapter

Use the [saved role defaults](subagents.md) through the Agent tool: pass the saved type as `subagent_type`, the model alias as `model`, and the effort level as `effort`. The saved defaults count as the explicit choice the `effort` field requires for a named level. Do not substitute `lower` or `higher` for the saved level.

Use `general-purpose` for every role. The built-in `Explore` type cannot use the Write or Edit tools, and every child writes its report and evidence files. Role boundaries such as "do not edit source" are enforced only by the brief.

Every round requires a fresh child. Do not use `subagent_type: "fork"`, which inherits the parent's conversation, and do not use SendMessage to continue or answer an existing child; a child that needs information ends its attempt as `blocked`. Children run in the background and the harness notifies the parent when each finishes. Wait for that notification; do not poll the child or predict its result.

Create worktrees with Git as `SKILL.md` describes, not with the Agent tool's `isolation: "worktree"`: the brief must name the worktree's path, branch, and starting revision before dispatch, and the tool creates its worktree only at launch.

Confirm the active tools support the assigned work, and report any model or type override imposed by the client. See the [Claude Code subagents documentation](https://code.claude.com/docs/en/sub-agents) for custom agent definitions.

## Journal

No journal hook is configured for Claude Code, so `attempt.finished` events are absent; do not record them yourself. `journal-append` records `$CLAUDE_CODE_SESSION_ID` as the `session.started` conversation ID. The parent transcript is `~/.claude/projects/<project-slug>/<session-id>.jsonl`, and child transcripts are under `<session-id>/subagents/`.
