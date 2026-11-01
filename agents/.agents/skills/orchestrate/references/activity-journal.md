# Activity journal

Record orchestration history for fresh-child handoffs and metrics. Continue using direct agent communication during execution.

## Identity and storage

- Reuse `task_id` across conversations and runs for the user-agreed outcome and acceptance criteria. PRs are linked artifacts, not task identities.
- Give each execution a `run_id` belonging to one task. Retain it on resume only when recorded history establishes continuity.
- Keep `work_item_id` stable for a bounded assignment goal. Give each fresh child assignment a unique `attempt_id` and each writer an `actor_id`.

Write to `<worktree-root>/.scratch/activity/runs/<run_id>/<actor_id>.jsonl`, one writer per stream. Resolve the root from the source checkout and pass absolute paths. Children in another worktree use that worktree's journal with the same task/run IDs. Use filesystem-safe unique IDs and create only current-run storage. Do not commit or publish journal data unless requested.

Append one JSON object per line. Preserve prior records; corrections reference the original event ID. When reading history, deduplicate by `event_id` and use causal references for ordering across writers. Report malformed records and failed writes as gaps, without blocking unrelated work.

## Events

Every event uses this envelope:

| Field | Value |
| --- | --- |
| `schema_version` | `1` |
| `event_id` | Unique ID, preserved on retry |
| `occurred_at` | UTC RFC 3339 timestamp |
| `type` | Event name below |
| `task_id`, `run_id`, `actor_id` | Task, execution, and writer IDs |
| `origin` | Object with `repository_id`, absolute `worktree_path`, absolute `journal_root` |
| `work_item_id`, `attempt_id` | Assignment IDs; null for task/run events |
| `caused_by` | Prior event IDs; empty when none |
| `data` | Event-specific fields below |

Use the canonical remote for `repository_id`, or a stable local identity when unavailable. Record unknown metadata as null.

The orchestrator records:

| Event | Data |
| --- | --- |
| `task.started` | Title, outcome, acceptance criteria; once per task |
| `run.started` | Harness, model/effort, native session ID, skill revision, working directory, branch, starting commit, existing changes |
| `attempt.dispatched` | Child actor, role, model/effort, native child ID if available, self-contained brief, report expectations, prior attempt/handoff references |
| `attempt.reviewed` | Decision (`accepted`, `correction_required`, `blocked`), rationale, evidence, correction instructions if needed |
| `artifact.linked` | Kind, relationship to task, stable locator such as PR URL, commit hash, issue URL, or report path |
| `run.finished` | Outcome (`completed`, `blocked`, `interrupted`), delivered work, criteria met/unmet, baseline/final check evidence, remaining work, journal/usage gaps |
| `task.completed` / `task.blocked` | Accepted outcome or blocker, evidence, artifact references |

A completed run need not complete its task. Link PRs only from known evidence, not branch-name inference.

Children record `attempt.finished`: status (`reported_complete`, `blocked`, `failed`, `interrupted`), changed files or findings, decisions, artifacts, evidence, blockers, and handoff summary. Check evidence includes assigned commands, exit statuses, exact failing output or its durable artifact, generated artifacts, and working-tree changes. Child completion requires a separate parent review for acceptance.

If a child cannot write, it returns its report directly. The parent records `attempt.observed` in its own stream, identifying the child and the source (child report or harness status). Preserve observed failed/interrupted attempts; a missing result is not success.

For corrections, include a review `reason`: `missing_context`, `unclear_scope`, `implementation_defect`, `missing_research`, `environment_issue`, `changed_requirement`, or `unknown`. Explain the assessment and link the next dispatch to that review.

## Handoffs

Record dispatch before spawning when storage is available. Include task/run/work-item/attempt and actor IDs, destination worktree, absolute stream path, and the envelope and `attempt.finished` fields in the child's brief. Request its result event ID/path alongside its report. Journal writes are report artifacts allowed for all roles; existing source-edit and check boundaries still apply.

Keep briefs self-contained. Link relevant prior results and reviews by event ID and absolute stream path; include needed evidence directly if the source is unavailable. Preserve decisions from direct communication in the next result or review with their source. History supplies context, not assignments or user approval.

## Metrics and usage

Count each dispatched implementer attempt as one implementation round for its work item; identify attempts with no observed execution/result. Keep research and checks separate. Label sums across work items as total implementation attempts. Derive correction counts and first-attempt acceptance from parent reviews.

A collector or agent with runtime evidence records `usage.recorded` with:

- Subject actor, native session/thread/turn IDs, and task/run/attempt attribution.
- Source record ID or locator, measurement interval, and incremental/cumulative counter semantics.
- Input, cached input, output, reasoning, and total tokens as exposed; unsupported counters are null.
- Whether coverage is partial/complete and includes children.

Use only measured runtime evidence, never estimates or unsupported child claims. Preserve the source's counter semantics: do not sum cumulative snapshots, count token subsets twice, or add children already included in parent totals. Leave ambiguous usage unattributed; cross-task usage requires runtime boundaries. Report unavailable usage. Final usage may be appended after `run.finished`.
