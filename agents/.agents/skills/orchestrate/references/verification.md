# Verification procedure

## Select evidence before implementation

Map each acceptance criterion to a configured command, an explicit behavioral scenario, or parent review of a specific artifact. Record this mapping in a short verification plan under the run directory and provide each child only its assigned part. Follow repository-required checks; identify coverage gaps rather than assuming lint, types, or an existing test suite proves the requested behavior.

For commands, specify exact invocations, working directory, and setup, including state and artifact inspection commands needed for the report. For UI, manual, or external-integration scenarios, specify the target, initial state, steps, expected observations, and evidence to capture, such as screenshots or responses. Use only available tools and authorized actions; missing capabilities or access are blockers. Observations that do not exercise a criterion leave it unverified.

Delegate applicable baseline checks to a fresh explorer before implementation, against the relevant starting state. Identify new scenarios that cannot run before the change, or why a baseline is inapplicable; do not manufacture a failing baseline for absent functionality. If execution adds no useful evidence for a documentation-only change, use explicit parent review criteria and required repository checks instead.

## Final verification and scope changes

After integration, delegate final commands and scenarios to a fresh explorer in the integration checkout. Preserve comparable baseline commands, setup, and scenarios where still relevant. Add checks for new behavior and changed integration paths. Record any additions, replacements, or omissions and their coverage rationale in the verification plan before dispatch. Do not drop required checks or redefine expected behavior to hide failures.

Review the [report and evidence](reports.md) against the criterion mapping. Compare baseline and final results, separating pre-existing failures from regressions and environment differences. Final verification must cover the integrated source state, not just individual task branches. Confirm meaningful observations rather than relying solely on a success exit status.

When code or check-generated files change after verification, identify which evidence is invalidated and repeat affected checks. Keep unaffected evidence only when its source state, setup, and coverage remain applicable. Expand verification when failures or newly discovered interactions justify it, rather than repeating every check automatically.

## Acceptance

The parent owns acceptance. A child's `complete` status means its assignment was performed, not that the change is correct. Passing commands support only the behavior they exercise; parent review must still address the mapped criteria.

Failed, skipped, interrupted, unrun, or unsupported required verification remains a reported limitation or blocker. Do not silently treat baseline failures as acceptable final passes, weaken checks, or leave criteria unmapped. Report what was verified, what remains unverified, and why.
