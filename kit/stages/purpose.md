# 0 Purpose

## Inputs

The task, users of the change and its known purpose, and the plan's completion conditions with IDs and their grounds.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Users and purpose of the change, and grounds for the plan's completion conditions. Restate the conditions in users' words; if changes are needed, propose replanning (`docs/architecture/data.md#%E8%A8%88%E7%94%BB%E3%81%AE%E3%83%90%E3%83%BC%E3%82%B8%E3%83%A7%E3%83%B3`).

## Deferred to later stages

All means of achieving the purpose.

## Irreversible items

None.

## Artifacts

An explanation of how the purpose corresponds to the completion conditions (`docs/checks.md#%E7%9B%AE%E7%9A%84`). A replanning proposal if changes are needed.

## Completion conditions

Explain the relationship between the purpose and completion conditions in users' words so completion can be judged. If completion conditions need changing, propose replanning rather than treating the change as already made.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6` for finalization responsibilities.
