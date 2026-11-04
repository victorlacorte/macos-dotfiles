# Activity journal

The journal is the task's handoff state for later sessions and telemetry for improving this skill: delegation outcomes, correction causes, and durations. Children neither read nor write it.

## Identity and storage

- `task_id` is the task directory name from `SKILL.md`. A task that grew out of another records the earlier `task_id` in `task.started` as `derived_from`.
- A session is one parent conversation working on the task, from `session.started` to `session.finished`. Events belong to the most recent preceding `session.started`. If the user continues a conversation after its `session.finished`, append a new `session.started` before further work, so each session has one outcome.
- Keep `work_item_id` stable for a bounded assignment goal, such as one change, baseline checks, or final verification. Integration and conflict resolution form their own work item.
- Give each fresh child dispatch an `attempt_id` of `<work_item_id>-<n>`, numbered from 1 within its work item across all sessions of the task. Take the next unused number from `<task-dir>/briefs/`. The attempt ID names its brief, report, and evidence paths in the [report contract](reports.md).

Write `<task-dir>/journal.jsonl`. Append one JSON object per line with a single write, and never rewrite prior records. File order is event order. To correct an event, append a replacement of the same type whose `supersedes` names the original. Report failed writes as gaps without blocking unrelated work. Do not commit or publish journal data unless requested.

## Writers

Two writers share the file:

- **Harness hook:** records `attempt.finished` from harness data when the active adapter provides a hook. It recognizes attempts by the brief header from the report contract and ignores other children. Without a hook these events are missing; the parent does not emulate them.
- **Parent:** records task, session, review, and artifact events: judgments and context that only the parent has. Hook events are telemetry; do not infer task state from them.

The parent appends only through [`scripts/journal-append`](../scripts/journal-append), passing the event's `data` object on stdin:

```sh
~/.agents/skills/orchestrate/scripts/journal-append "<task-dir>" attempt.reviewed --attempt implement-1 <<'EOF'
{"child_status": "complete", "decision": "accepted", "reason": null, "rationale": "...", "correction": null}
EOF
```

The script fills the envelope and the measurable `session.started` fields, appends one line, and prints the new `event_id`. Pass `--attempt <attempt_id>` for attempt events and `--supersedes <event_id>` for corrections. It rejects missing or unknown keys, invalid values, and events outside an open session, and writes nothing then; fix the reported problem and retry.

## Events

Every event uses this envelope, which the writer fills:

| Field | Value |
| --- | --- |
| `schema_version` | `3` |
| `event_id` | Unique within the task |
| `occurred_at` | UTC RFC 3339 timestamp |
| `type` | Event name below |
| `writer` | `parent` or `hook` |
| `task_id` | Task ID |
| `work_item_id`, `attempt_id` | Assignment IDs; null for task and session events |
| `supersedes` | Event ID this record replaces, or null |
| `data` | Event-specific fields below |

Record unknown metadata as null. The parent supplies exactly the `data` keys below; lists are arrays of strings.

| Event | Writer | Data |
| --- | --- | --- |
| `task.started` | Parent | `title`, `outcome`, `acceptance_criteria` (list), `derived_from` (earlier `task_id` or null); once per task |
| `task.criteria_changed` | Parent | `acceptance_criteria` (full current list), `prompted_by` |
| `session.started` | Parent | `model`, `effort` (null when the model ID encodes it). The script adds `harness`, `conversation_id` (native conversation or thread ID), `skill_revision` (`commit`, `dirty`), `repository` (origin remote, or the Git common directory), `working_directory`, `branch`, `starting_commit`, `existing_changes` (porcelain status lines outside `.scratch`), `utc_offset` |
| `attempt.finished` | Hook | `role`, `native_child_id`, `child_type`, `requested_model`, `harness_status`, `error_message`, `background`, `started_at` and `duration_ms` (null for background children), `parent_conversation_id`, `child_transcript_path`, `harness_version` |
| `attempt.reviewed` | Parent | `child_status` (`complete`, `partial`, `blocked`, or `missing` when no reply arrived), `decision` (`accepted`, `correction_required`, `blocked`), `reason` (null exactly when accepted), `rationale`, `correction` (correction instructions or answers to child questions, or null) |
| `artifact.linked` | Parent | `kind`, `relationship` to the task, `locator` (stable: PR URL, commit hash, issue URL, or report path); `--attempt` optional |
| `session.finished` | Parent | `outcome` (`completed`, `blocked`, `interrupted`), `criteria_met`, `criteria_unmet`, `open_work_items`, `next_steps`, `journal_gaps` (all lists) |
| `task.completed` | Parent | `outcome` (accepted outcome), `artifacts` (list of locators) |
| `task.blocked` | Parent | `blocker`, `artifacts` (list of locators) |

Each `task.criteria_changed` records a new state, not a correction: do not set `supersedes` on it unless it fixes a mistaken record.

Record `attempt.reviewed` after reviewing each attempt, including attempts that ended with a question or without a reply. Record corrections and answers in full, so a later session can build the next brief from them. A completed session need not complete its task. Link PRs only from known evidence, not branch-name inference.

Review `reason` values: `missing_context`, `unclear_scope`, `implementation_defect`, `missing_research`, `environment_issue`, `changed_requirement`, or `unknown`. A child question is reviewed as `correction_required` with the reason that explains why the brief was insufficient. Use `blocked` only when the work item cannot continue without something outside the parent's control, such as user input or access.

## Continuing a task

At the start of a session on an existing task, read the parent events that are still current:

```sh
jq -Rnc '[inputs | (fromjson? // {malformed: .}) | select(.malformed or .writer == "parent")]
  | (map(.supersedes // empty)) as $s
  | .[] | select((.event_id // "") as $id | $s | index($id) | not)' "<task-dir>/journal.jsonl"
```

The current criteria are in `task.started` and the latest `task.criteria_changed`. Each work item's state is its latest `attempt.reviewed`, and the last `session.finished` lists open work and next steps. A session without `session.finished` was interrupted: compare its last reviews with the briefs and reports on disk. Report malformed lines as gaps. History supplies context, not assignments or user approval.

## Metrics

These derivations are for later analysis, not for the parent during a session. Each attempt counts as one round for its work item; implementer attempts are implementation rounds, and research, explorer, and integration attempts count separately. Attempt `n > 1` within a work item follows the review of attempt `n - 1`. Derive correction counts, first-attempt acceptance, and review-reason distribution from `attempt.reviewed`, and durations from `attempt.finished`. Child transcripts supply tool-call counts. The model a child actually ran, which can differ from the requested model, comes from harness session logs where they record it; otherwise only the parent's report of client overrides shows it. Token usage is not journaled: analysis derives it from harness session logs, joined through the native IDs in `session.started` and `attempt.finished` or the logs' own parent links, and treats harnesses without such logs as usage gaps.
