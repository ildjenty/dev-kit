# 0 Purpose

## Inputs

The task, users of the change and its known purpose, and the plan's completion conditions with IDs and their grounds.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Users and purpose of the change, and grounds for the plan's completion conditions. Restate the conditions in users' words; if changes are needed, propose replanning following `plans/README.md`.

## Deferred to later stages

All means of achieving the purpose.

## Irreversible items

None.

## Artifacts

An explanation of how the purpose corresponds to the completion conditions (`checks/purpose.md`). A replanning proposal if changes are needed.

## Completion conditions

Explain the relationship between the purpose and completion conditions in users' words so completion can be judged. If completion conditions need changing, propose replanning rather than treating the change as already made.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
