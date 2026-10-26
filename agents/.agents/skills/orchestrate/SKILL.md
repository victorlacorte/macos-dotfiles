---
name: orchestrate
description: Delegate implementation, research, codebase exploration, and configured checks to fresh subagents, then review their changes and evidence.
disable-model-invocation: true
---

# Orchestrate

Coordinate work through bounded subagent tasks. The parent owns acceptance criteria, local review, and interpretation of check evidence. Do not perform implementation, external research, or assigned check execution yourself under this skill.

## Before dispatch

Read the canonical [subagent policy](references/subagents.md) and the active harness adapter: [Cursor](references/cursor.md) or [Codex](references/codex.md). Use only the adapter for the current harness. These are saved instructions for the orchestrator, not settings installed into the harness. Follow the policy for resolving and checking configured choices.

## Workflow

1. Define acceptance criteria and the quality bar from the request and repository guidance. Inspect relevant files, conventions, setup, and scope. Resolve material ambiguities from available context.
2. When checks are in scope, derive the configured check commands and delegate a baseline run to a fresh explorer. Provide exact commands, working directory, required environment or setup, and ask for each exit status and exact failing output. Use the baseline results in the implementation brief.
3. Delegate external facts to a researcher, codebase mapping to an explorer, and bounded changes to an implementer. Run independent tasks in parallel only when their work and writes are disjoint.
4. Start every round with a fresh child and clean context. Give each child a self-contained brief with its goal, acceptance criteria, scope, constraints, relevant findings, prior work to preserve, and requested report. Implementers may read sources and diffs, but do not run checks or external research; they should request missing research. Explorers assigned checks run only the listed commands, do not edit source or fix failures, and report status, exact failing output, generated artifacts, and working-tree changes. Check runs may create artifacts, so do not require blanket read-only mode.
5. Review implementation diffs and relevant files against the criteria. Send concrete corrections to a fresh implementer when needed. Reassess assumptions when evidence contradicts them, and stop on a real blocker rather than repeating unchanged attempts.
6. After the final diff, delegate the same required checks to a fresh explorer with the exact commands and setup. Compare results with baseline, inspect generated artifacts and working-tree changes, and interpret failures yourself. If checks fail, correct through a fresh implementer or revisit a flawed assumption, then request a fresh final check. Do not weaken checks to obtain a pass.
7. Finish when criteria are met and final verification passes, or report the remaining blocker and its evidence.

## Final report

Report what changed, the number of implementation rounds, final check results reported by the explorer, and unresolved issues.
