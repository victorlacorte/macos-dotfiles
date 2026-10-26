# Saved subagent policy

These are saved defaults for the orchestrator to apply when dispatching children. The harness does not load or install them automatically. They configure children, not the orchestrator. When the user explicitly invokes this skill, that invocation selects its saved role, type, model, and effort defaults unless the user supplies overrides. For purposes of applicable confirmation instructions, this counts as the user's choice of those configured settings, while higher-priority instructions still take precedence. Apply higher-priority instructions and explicit user choices first, then these defaults, then compatible native defaults only for unspecified fields. Keep per-session overrides ephemeral unless the user asks to save them. Do not ask the user to repeat a complete configured choice.

Check the resolved model, effort, type, and capabilities against the active spawn tool. A generic spawn tool is a mechanism, not a named native agent type. Use a named native definition only when the tool supports it and the user or policy selects it. If a resolved choice is unavailable, report the blocker instead of substituting another model or type. A harness without saved defaults here requires its own policy or complete explicit supported choices; never transfer another harness's defaults.

## Saved defaults

| Harness | Role | Type or mechanism | Model | Effort |
| --- | --- | --- | --- | --- |
| Cursor | Implementer | `generalPurpose` | `cursor-grok-4.6-medium` | Encoded in model ID |
| Cursor | Researcher | `explore` | `cursor-grok-4.6-medium` | Encoded in model ID |
| Cursor | Explorer | `explore` | `cursor-grok-4.6-medium` | Encoded in model ID |
| Codex | Implementer | Generic spawn agent | `gpt-6-luna` | `medium` |
| Codex | Researcher | Generic spawn agent | `gpt-6-luna` | `medium` |
| Codex | Explorer | Generic spawn agent | `gpt-6-luna` | `medium` |

Use the active harness adapter to map these choices to supported tool fields.

## Role boundaries

- **Implementer:** Makes assigned changes. May read local sources and diffs. Does not run checks or external research; asks the parent to arrange missing research. Reports changed files, decisions, and blockers.
- **Researcher:** Answers the assigned external question using available research sources. Prefers official primary sources and reports links or citations, distinguishing confirmed facts from unknowns. Does not implement.
- **Explorer:** Maps the codebase or runs explicitly assigned cheap, deterministic checks, such as typechecking, lint, builds, tests, and formatting checks. For checks, runs only listed commands and reports each result, generated artifacts, and working-tree changes. Does not edit source or fix failures.

The parent reviews implementation diffs and interprets check evidence. Delegate configured baseline and final checks to fresh explorers when checks are in scope. Do not add a verifier role. Check-generated artifacts are not source edits; report them separately.
