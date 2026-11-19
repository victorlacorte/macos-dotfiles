---
name: reviewable-prs
description: Deliver new coding work as small, independently reviewable pull requests, branching from the base for independent slices and using gh-stack for dependent ones.
disable-model-invocation: true
---

# Reviewable PRs

Split new coding work into pull requests that a human can review one at a time. This skill does not rescue an existing oversized branch or pull request.

## Map the slices

Before editing, inspect the request, repository conventions, and working tree. Use the base branch the user names, otherwise the trunk. Preserve unrelated worktree changes: use isolated worktrees or stage only the slice's files.

A small task is one branch and one pull request. For larger work, map every slice before writing code, each with:

- a concrete outcome and acceptance criteria;
- its dependency and base branch;
- the checks that demonstrate the outcome.

Each slice delivers one demonstrable behavior. A foundation slice with no user-facing behavior needs checks showing it is ready for dependent work. Keep slices within the request: no unrelated cleanup, unrequested behavior, or speculative abstractions. If a slice spans multiple review concerns or needs a long explanation, split it and update the map. Treat diff size as a warning, not a limit.

Follow the user's delivery cadence; if none is given, build and publish all slices. Ask only when the request implies a review gate between slices without defining one. Never merge.

## Choose the branch shape

- Independent slices: one branch per slice from the base, each pull request targeting the base.
- Dependent slices: a `gh stack` chain. Do not stack slices that could target the base independently.
- Never combine independent slices on a shared feature branch, and never hand-build parent-branch pull request chains instead of `gh stack`.

## Stack prerequisites

Before stacking, confirm the `gh-stack` GitHub CLI extension is installed and its `gh-stack` skill is readable, then follow that skill for repository support checks and all stack mechanics. If a prerequisite is missing or the repository rejects stacks (including exit code 9), report the blocker and stop stack work: do not install or vendor anything, and do not fall back to manual chains or a shared branch. Independent pull requests still proceed.

Keep one slice per stack layer and publish non-interactively when the cadence calls for it.

## Per slice

1. Implement only the stated outcome and run its checks.
2. Diff against the target branch; remove scope drift, or split if the diff covers multiple concerns.
3. Write the description after verification: behavior, reason, dependency or base, and checks. Open the pull request.
4. When addressing review feedback, change the affected slice and rerun its checks.

The reviewer, not this skill, decides approval.
