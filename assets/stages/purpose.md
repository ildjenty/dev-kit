# 0 Purpose

## Inputs

The task, users of the change and its known purpose, existing `requirements.md`, and the plan's human-approved completion conditions with IDs and their grounds.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Ask about matters the request leaves undecided: purpose, users, main usage scenarios, data, runtime environment, authentication and permissions, scale, performance, security, existing integrations, exclusions, deadlines, and priorities. List provisional assumptions and confirm them with the human. Reconcile the answers with existing requirements and agree on completion conditions written as concrete usage examples, with IDs and grounds.

## Deferred to later stages

All means of achieving the purpose.

## Irreversible items

None.

## Artifacts

An explanation of how the purpose corresponds to the proposed completion conditions (`principles/review/purpose.md`), the agreed proposal with IDs and grounds, the confirmed assumptions, and `requirements.md` updated with requirements clarified with the human. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Explain the relationship between the purpose and proposed completion conditions in users' words so completion can be judged, confirm the requirements and proposal with the human, and reflect the agreed repository requirements in `requirements.md`. If the plan's `done` transition for this stage is `replan`, the planner rebuilds the remaining plan after this stage completes. Changes to completion conditions require human approval following `plans/README.md`; do not treat the proposal as the active plan.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
