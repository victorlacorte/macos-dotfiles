Codex uses the versioned `general` profile to grant the sandbox permission
needed to persist Plan Mode handoffs in `~/.codex/plans/`. The profile overlays
the private `~/.codex/config.toml`; it does not replace or manage that file.

The shell wrapper in `zsh/.zshrc` adds `--profile general` when no explicit
profile is provided. It preserves `--profile`, `--profile=...`, and `-p`
arguments. Use `command codex ...` to bypass the wrapper when needed. Create
`~/.codex/plans/` before using the profile if it does not exist.

This `AGENTS.md` documents the required permission and workflow, but it cannot
grant sandbox access by itself.

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
