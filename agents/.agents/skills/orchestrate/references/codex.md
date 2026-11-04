# Codex adapter

Use the [saved role defaults](subagents.md) and the fields exposed by the active spawn tool. For the generic spawn tool, put the role and task in the message. Use `task_name` only as a label, and pass the resolved `model` and `reasoning_effort` explicitly. Set `fork_turns: "none"` when that field is exposed. Do not invent an `agent_type` or assume a task label selects a named agent definition. Do not send input to or resume an existing child; every round requires a fresh one.

Check that the child has the tools needed for its task. If the tool cannot express a required choice or capability, report the limitation instead of silently changing the selection. See the [Codex subagents documentation](https://learn.chatgpt.com/docs/agent-configuration/subagents) for native agent configuration.

No journal hook is configured for Codex, so `attempt.finished` events are absent; do not record them yourself. `journal-append` records `$CODEX_THREAD_ID` as the `session.started` conversation ID; it names the session log `~/.codex/sessions/YYYY/MM/DD/rollout-*-<thread-id>.jsonl`, from which analysis derives token usage.
