# 0 Purpose

## Inputs

The task, users of the change and its known purpose, existing `requirements.md`, and the plan's human-approved completion conditions with IDs and their grounds.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Clarify the users, purpose, and requirements of the change with the human instead of guessing. Reconcile them with existing requirements and agree on a proposal for completion conditions written in users' words, with IDs and grounds.

## Deferred to later stages

All means of achieving the purpose.

## Irreversible items

None.

## Artifacts

An explanation of how the purpose corresponds to the proposed completion conditions (`checks/purpose.md`), the agreed proposal with IDs and grounds, and `requirements.md` updated with requirements clarified with the human. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

Explain the relationship between the purpose and proposed completion conditions in users' words so completion can be judged, confirm the requirements and proposal with the human, and reflect the agreed repository requirements in `requirements.md`. If the plan's `done` transition for this stage is `replan`, the planner rebuilds the remaining plan after this stage completes. Changes to completion conditions require human approval following `plans/README.md`; do not treat the proposal as the active plan.

All required checks valid for the returned target and norms are `PASS`, the detection requirements in `checks/README.md#detection-tests` are satisfied, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
