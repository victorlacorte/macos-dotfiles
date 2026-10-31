---
name: orchestrate
description: Delegate implementation, research, codebase exploration, and configured checks to fresh subagents, then review their changes and evidence.
disable-model-invocation: true
---

# Orchestrate

Coordinate bounded subagent tasks to reduce expensive parent work and keep noisy exploration out of its context while meeting the required quality bar. The parent owns acceptance criteria, local review, and interpretation of evidence. Delegate implementation, external research, and assigned verification.

## Before dispatch

Read the canonical [subagent policy](references/subagents.md) and the active harness adapter: [Cursor](references/cursor.md) or [Codex](references/codex.md). Use only the adapter for the current harness. These are saved instructions for the orchestrator, not settings installed into the harness. Follow the policy for resolving and checking configured choices.

## Delegation and context

Delegate broad or iterative codebase searches, URL retrieval and source comparison, and large log analysis. Give one explorer or researcher a bounded question and let it perform the related searches and filtering. Do not spawn a child for each grep or URL. Require concise conclusions with file references or citations, uncertainty, and links to evidence.

Children should narrow paths and list matching files before reading contents, use bounded excerpts, and retrieve relevant source sections rather than entire payloads when supported. Store large raw outputs as evidence. Filtering should reduce the child's context too, not merely move noisy output out of the parent.

The parent may read instructions, inspect repository state, locate a known file with a bounded search, and read targeted files, reports, and diffs for dispatch or acceptance. Do not repeat a child's exploration before dispatch or during review; follow its references and investigate concrete uncertainties. Implementers may search within their assigned scope without a separate explorer.

Use the configured child models and fresh, self-contained briefs. Assign non-overlapping questions and pass established findings forward. Batch closely related small operations; do not delegate trivial parent bookkeeping or known-file lookups. Delegation reduces parent context exposure, but startup, briefing, child work, and review can increase total tokens. Do not assume subsidy or monetary savings without applicable pricing or usage evidence.

## Run directory and handoff

Before dispatch, create `<project-root>/.scratch/<YYYYMMDD-HHMM>-<subject-slug>/`, using local time and a lowercase kebab-case subject. Add a numeric suffix if the path belongs to another run. Reuse this directory throughout the orchestration task. Keep artifacts uncommitted and exclude `.scratch` from code searches and source-change review.

Use the [Markdown report contract](references/reports.md). Pass the absolute run directory, assigned report and evidence paths, working directory, and contract in every brief; children do not inherit this skill. All children, including those in worktrees, write reports to this canonical run directory. Keep artifacts available through final review.

Children return only the report path, completion status (`complete`, `partial`, or `blocked`), and at most three summary bullets in chat. Read each report before acceptance; open supporting evidence as needed. Investigation output stays in files until relevant to a parent decision.

## Parallel work and worktrees

Use separate Git worktrees for parallel implementation tasks when their changes can be developed independently. Read-only exploration can share a checkout. Without worktree support, parallelize only disjoint writes; serialize overlapping or dependent changes. Separate worktrees isolate files, not shared services, ports, databases, or build caches.

Record each worktree's absolute path, branch or detached state, and starting revision; provide them in the brief. Preserve existing user changes: a new worktree does not include uncommitted edits, so arrange the required starting state explicitly or keep dependent work in the original checkout. Use one writer per worktree and do not change another task's branch or files.

Children report their base revision, resulting commits or patch, and integration dependencies. Review their changes and delegate integration and conflict resolution to an implementer. Verify the combined result in the integration checkout before acceptance; individual worktree passes do not verify the combined result. Keep worktrees until their changes are integrated or handed off and their evidence reviewed.

## Workflow

1. Define acceptance criteria, scope, and the quality bar from the request and repository guidance. Delegate discovery needed to resolve material ambiguities.
2. Map each criterion to evidence using the [verification procedure](references/verification.md). Delegate applicable baseline checks to a fresh explorer before implementation. Use findings in the implementation brief, linking logs instead of copying them.
3. Delegate external facts to a researcher, codebase mapping to an explorer, and bounded changes to an implementer. Partition responsibilities and use worktrees where appropriate.
4. Start every round with a fresh child and clean context. Brief it with the goal, acceptance criteria, owned scope, constraints, relevant findings, prior work to preserve, working state, artifact paths, and report contract. Implementers do not run verification or external research; they request missing evidence. Explorers execute only the assigned commands or behavioral procedures and do not fix failures. Checks may create artifacts, so do not require blanket read-only mode.
5. Read reports and review diffs and relevant files against the criteria. Completion status is not acceptance or proof of verification. Send concrete corrections to a fresh implementer. Reassess contradicted assumptions and stop on real blockers rather than repeating unchanged attempts.
6. After integration and the final diff, delegate final verification to a fresh explorer. Review its report and evidence against the verification plan and the final source state. Route necessary fixes through an implementer, then repeat affected verification.
7. Finish when criteria and required verification are satisfied, or report the remaining blocker and evidence. Pre-existing failures, regressions, skipped checks, and unrun checks must remain distinguishable; none count as a pass.

## Final report

Report what changed, the number of implementation rounds, verification results and their source (explorer execution or parent artifact review), and unresolved issues. Link the relevant reports and evidence, and state verification limits explicitly.
