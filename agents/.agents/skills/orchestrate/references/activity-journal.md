# Activity journal

Record orchestration history for fresh-child handoffs and metrics. Only the parent writes the journal. Children return Markdown reports, and the parent records their results.

## Identity and storage

- Reuse `task_id` across conversations and runs for the user-agreed outcome and acceptance criteria. PRs are linked artifacts, not task identities. To continue a task, use the run the user names or search prior `.scratch/*/journal.jsonl` files for its `task_id`.
- `run_id` is the run directory name from `SKILL.md`. Each run belongs to one task. Continue an existing run only when its recorded history establishes continuity; otherwise start a new run.
- Keep `work_item_id` stable for a bounded assignment goal, such as one change, baseline checks, or final verification. Integration and conflict resolution form their own work item.
- Give each fresh child dispatch an `attempt_id` of `<work_item_id>-<n>`, numbered from 1 within its work item. It names the attempt's brief, report, and evidence paths in the [report contract](reports.md).

Write `<run-dir>/journal.jsonl`. Append one JSON object per line and never rewrite prior records; corrections are new events that reference the original in `caused_by`. File order is event order. Report malformed records and failed writes as gaps without blocking unrelated work. Do not commit or publish journal data unless requested.

## Events

Every event uses this envelope:

| Field | Value |
| --- | --- |
| `schema_version` | `2` |
| `event_id` | Unique within the run |
| `occurred_at` | UTC RFC 3339 timestamp |
| `type` | Event name below |
| `task_id`, `run_id` | Task and run IDs |
| `work_item_id`, `attempt_id` | Assignment IDs; null for task/run events |
| `caused_by` | Prior event IDs; empty when none |
| `data` | Event-specific fields below |

Record unknown metadata as null.

| Event | Data |
| --- | --- |
| `task.started` | Title, outcome, acceptance criteria; once per task |
| `run.started` | Harness, model/effort, native session ID, skill revision, repository ID (canonical remote, or a stable local identity), working directory, branch, starting commit, existing changes, local UTC offset |
| `attempt.dispatched` | Role, model/effort, brief path, working directory or worktree path, starting revision, report and evidence paths |
| `attempt.finished` | Status, source (`child_reply` or `harness_status`), native child ID if available, report path, changed files, blockers |
| `attempt.reviewed` | Decision (`accepted`, `correction_required`, `blocked`), `reason` unless accepted, rationale, evidence links, correction instructions or answers |
| `artifact.linked` | Kind, relationship to task, stable locator such as PR URL, commit hash, issue URL, or report path |
| `run.finished` | Outcome (`completed`, `blocked`, `interrupted`), delivered work, criteria met/unmet, baseline/final check evidence, remaining work, journal/usage gaps |
| `task.completed` / `task.blocked` | Accepted outcome or blocker, evidence, artifact references |

A completed run need not complete its task. Link PRs only from known evidence, not branch-name inference.

The `attempt.finished` status is the child's reported `complete`, `partial`, or `blocked`. Use `failed` or `interrupted` when the harness reports that the child did not finish, and `missing` when no result arrived; none of these is success. The report remains the source of findings and evidence; the event records only what metrics and later briefs need to locate.

Review `reason` values: `missing_context`, `unclear_scope`, `implementation_defect`, `missing_research`, `environment_issue`, `changed_requirement`, or `unknown`. A child question is reviewed as `correction_required` with the reason that explains why the brief was insufficient. Use `blocked` only when the work item cannot continue without something outside the parent's control, such as user input or access.

## Handoffs

Write the brief to its path and record `attempt.dispatched` before spawning; link it through `caused_by` to the review that prompted it, if any. Children do not read or write the journal, so keep briefs self-contained. For a correction or answered question, build the next brief from the previous brief, the prior report, and the review: describe the current working state, the partial results to keep, and the corrections or answers. History supplies context, not assignments or user approval.

When the child returns, record `attempt.finished` from its reply and report, then `attempt.reviewed` after review.

## Metrics and usage

Count each implementer attempt as one implementation round for its work item, including attempts that ended with a question; identify attempts without a result. Count research, explorer, and integration attempts separately. Label sums across work items as total implementation attempts. Derive correction counts, first-attempt acceptance, and the distribution of review reasons from `attempt.reviewed`. Attempt duration is the interval from `attempt.dispatched` to `attempt.finished`.

When a runtime source exposes token usage, the parent records `usage.recorded` with:

- Subject (orchestrator or attempt) and native session/thread/turn IDs.
- Source record ID or locator, measurement interval, and incremental/cumulative counter semantics.
- Input, cached input, output, reasoning, and total tokens as exposed; unsupported counters are null.
- Whether coverage is partial/complete and includes children.

Use only measured runtime evidence, never estimates or unsupported child claims. Preserve the source's counter semantics: do not sum cumulative snapshots, count token subsets twice, or add children already included in parent totals. Leave ambiguous usage unattributed; cross-task usage requires runtime boundaries. When no source exposes usage, record that once as a gap in `run.finished`. Final usage may be appended after `run.finished`.
