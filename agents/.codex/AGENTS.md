Read and follow the instructions in `~/AGENTS.md`. These instructions apply in addition to the rules in this file; where they conflict, this file takes precedence.

## Plan Handoff

Every decision-complete plan produced in Plan Mode must be persisted automatically. Do not wait for a follow-up request. Before presenting the final `<proposed_plan>`:

- Ensure `~/.codex/plans/` exists.
- Each planning thread has one canonical handoff file.
- For the first finalized plan, create: `~/.codex/plans/<YYYYMMDD-HHMM>-<subject-slug>.md`
- Derive `<subject-slug>` from the plan's subject or goal using lowercase kebab-case.
- If the generated path already belongs to another plan, add a numeric suffix when creating the file.
- When the plan is revised, update that same file in place. Do not create another file or preserve stale revisions.
- Keep the original filename stable even if the plan's subject or wording changes.
- Report the canonical file path whenever the plan is created or revised.

The saved plan must be self-contained for a fresh agent and include, when applicable:

- the repository's absolute path;
- current branch, base reference, and relevant working-state context;
- objective and success criteria;
- implementation decisions, interfaces, assumptions, constraints, and scope boundaries;
- migration, compatibility, or rollout requirements;
- tests and verification commands;
- unresolved blockers.

If the file cannot be created, clearly report the failure instead of claiming the plan was persisted.

## Subagent Selection

When asked to spawn subagents, always spawn Luna Max (gpt-5.6-luna with max reasoning) agents unless the user explicitly requests a different agent type.
