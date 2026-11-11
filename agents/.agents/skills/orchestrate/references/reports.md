# Markdown report contract

## Dispatch and handoff

The parent uses the task directory specified in `SKILL.md`. For each attempt, it writes the brief to `<task-dir>/briefs/<attempt_id>.md`, which is the canonical brief, and assigns `<task-dir>/reports/<attempt_id>.md` and `<task-dir>/evidence/<attempt_id>/`, using the `attempt_id` from the [activity journal](activity-journal.md). Worktree tasks use the task directory in the original project, not a new `.scratch` under their worktree. Paths must be writable by the child and readable by the parent; attempts must not share output paths. If artifact access is unavailable, report the blocker rather than dumping the investigation into chat.

Start every brief with this header, using absolute paths:

```text
Orchestrate attempt
Role: <implementer|researcher|explorer>
Report: <task-dir>/reports/<attempt_id>.md
Evidence: <task-dir>/evidence/<attempt_id>/
Working directory: <path>
```

Keep these exact labels on their own lines.

The dispatch prompt contains only the same five header lines, then the line `Read your full brief first and follow it exactly: <task-dir>/briefs/<attempt_id>.md`, then the final-reply requirements below, so they hold even if the child skims the brief. Harness hooks identify the task and attempt from the prompt's `Report:` line, so the header must be in the prompt itself, not only in the brief file.

Include these requirements in both the brief and the dispatch prompt. The child's final chat reply contains only:

- The absolute report path, or an explicit statement that the report could not be written.
- Completion status: `complete`, `partial`, or `blocked`.
- A summary of at most three bullets, including any blocker.

`complete` means the assigned work was performed and reported. For a check-running task, it does not mean the checks passed. If missing information or evidence prevents the assignment, the child stops, records its questions and any partial results in the report, and reports `blocked`. The parent answers through a new brief to a fresh child, never through a follow-up message.

## Report contents

Write only results relevant to the assigned task:

- **Outcome:** Completion status and outcome against each assigned acceptance criterion.
- **Findings and decisions:** Conclusions needed by the parent, including material uncertainty. Researchers include source links and distinguish confirmed facts from unknowns.
- **Files and evidence:** Changed files, relevant source references, and links to supporting artifacts. Identify the checkout and base revision; worktree implementers include resulting commits or a patch and integration dependencies. Use absolute paths or links relative to the report so evidence remains locatable.
- **Verification:** Check results when checks were assigned; otherwise state that checks were not assigned. Implementers label each listed fast check they ran, with its result, as a self-check; self-checks are not acceptance evidence. Implementers report pre-existing failures within the touched scope without fixing them unless the brief assigns them.
- **Blockers:** Unresolved questions, missing prerequisites, and work still needed, or `none`.

Omit investigation transcripts, intermediate reasoning, and raw command output. Keep lengthy supporting evidence in separate files and link it from the report. The report must state decisive findings and blockers without requiring the parent to reconstruct them from logs.

## Check evidence

For baseline and final verification tasks, record:

- The checked source state: commit and relevant uncommitted changes when applicable. Identify changes during the run that could affect the results.
- Each assigned command exactly as executed, or behavioral steps performed, its working directory or target, and relevant setup or environment differences. Do not record secret values. For CI-sourced checks, record instead the run URL, workflow, head commit, per-job results, and links to saved logs; record jobs that passed only on rerun as flaky.
- Each command or scenario's result: passed, failed, skipped, or not run. Include actual exit statuses for commands when available; explain skipped work, interrupted runs, and missing exit statuses. For behavioral checks, compare expected and observed results and link captured evidence.
- A concise failure description and links to full captured output, preserving exact failing output in evidence files. Save lengthy successful output separately as well.
- Generated artifacts and working-tree changes caused by the checks, separately from pre-existing changes and the report artifacts themselves.

Record every assigned command or scenario, including work never reached after an earlier failure. Do not infer a pass from a missing result. If evidence could not be captured, disclose the gap.

The parent reads the report, inspects relevant diffs and artifacts, and opens evidence needed to assess failures or material claims. Compare baseline and final results with attention to source and environment differences. A passing command supports only what it checks; the parent still reviews acceptance criteria against the implementation. Missing or unreadable reports must be recovered before accepting the task.
