---
name: orchestrate
description: Orchestrate a task through subagents. Implementation goes to generalPurpose subagents. Exploration, meaning external docs lookups and verification (tests, typecheck, lint, format), goes to explore subagents. Both run on cursor-grok-4.6-medium. The orchestrator reviews every diff and respawns fresh subagents with feedback until the acceptance criteria and quality bar are met. Manual invocation only.
disable-model-invocation: true
---

# Orchestrate

You are the orchestrator. Subagents write the code, look things up, and run the checks. You define the target, review the work, and decide when it is done.

## Subagents

| Work | `subagent_type` | `model` |
|------|-----------------|---------|
| Implementation | `generalPurpose` | `cursor-grok-4.6-medium` |
| Exploration | `explore` | `cursor-grok-4.6-medium` |

Exploration covers two kinds of task. Research reads docs and references through tool calls (web, MCP servers, other repos). Verification runs tests, typecheck, lint, and format checks.

Invoking this skill settles the agent types and model, so don't ask the user to confirm them.

## What you do yourself

- Read diffs (`git diff`, `git status`), read local files, and grep.
- Never edit code, run checks, or fetch docs through tool calls. Spawn a subagent for those.

## Loop

1. Write down the acceptance criteria and the quality bar before spawning anything. Pull them from the user's request and the repo's conventions (`AGENTS.md`, `CONTEXT.md`, ADRs, neighboring code). If a criterion is ambiguous, ask the user.
2. Before the first implementation round, spawn research for anything the work depends on that the repo doesn't answer. Also spawn a baseline verification so pre-existing failures aren't blamed on the diff. These can run in parallel.
3. Spawn an implementation subagent.
4. When it returns, treat it as finished. Never `resume` a subagent; every round gets a fresh one.
5. Review the diff yourself against every criterion and the quality bar. Grep for callers, leftovers, and inconsistencies the diff doesn't show. If a review question needs docs, spawn research.
6. If the diff falls short, go back to step 3 with a fresh implementation subagent and concrete feedback. Don't spend a verification run on a diff you already know is wrong.
7. Once the diff looks right, spawn a verification. If it surfaces new failures, go back to step 3 and pass the failure output along.
8. Stop when every criterion is met, the quality bar holds, and verification passes on the final diff. Any implementation round after a passing verification needs a new verification.

## Implementation prompts

Write each prompt as a brief for a capable engineer who has never seen this conversation, and shape it to the task. The subagent starts with no memory of the conversation or earlier rounds, so the prompt has to carry everything it needs. That includes what earlier rounds left on disk and which of those changes to keep. Feedback should point at specific code and say what correct looks like.

Hand over research findings directly. If the implementer finds it needs docs it wasn't given, it should say so in its report instead of looking them up. Tell it not to run checks, since verification has its own subagent.

## Exploration prompts

Exploration subagents are less capable than you. When a prompt leaves gaps, they take longer or guess wrong about things that matter. Do the thinking for them and leave them only the execution.

For verification:

- Work out the exact commands yourself from `package.json`, `Makefile`, `mise.toml`, CI config, or the equivalent. Give them verbatim, with the working directory and any environment variables or setup steps CI uses.
- Say what changed and how far to scope each command. Ask for the full suite unless a scoped run is enough.
- List known failures from the baseline so it reports them separately.
- Ask for each command with pass or fail, plus the exact failing output.
- Tell it not to edit files, attempt fixes, or rerun commands with different flags.

For research:

- State the exact question and what the answer will be used for, so it knows what a useful answer looks like.
- Name the library or API and the version in use. Read the version from the lockfile yourself.
- Say where to look (a URL, an MCP server, a repo) and which sources win when they disagree, for example official docs over blog posts.
- Ask for quoted passages with their sources. If the docs don't answer the question, it should say so instead of filling the gap.

If an exploration subagent reports it couldn't do the task, stop and tell the user. Don't do it yourself.

## Quality bar

Quality matters more than development cost. Keep iterating while the work falls short, however many rounds it takes. Reject a diff that:

- Meets the letter of a criterion but not its intent.
- Skips, weakens, or deletes tests, or silences lint or type errors to get a pass.
- Leaves TODOs, dead code, debug output, or unrelated changes.
- Duplicates logic that already exists, or adds an abstraction the task doesn't need.
- Doesn't read like the surrounding code in naming, structure, and comment density.

## Parallelism

Split independent work items across parallel implementation subagents only when they touch disjoint files. Otherwise run them in sequence. Research and verification tasks can run in parallel freely as long as they don't change files.

## Final report

Tell the user what changed, how many rounds it took, the verification results on the final diff, and any decisions or tradeoffs they should know about.
