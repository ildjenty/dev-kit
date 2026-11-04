# 4 Steps

## Inputs

The plan's completion conditions with IDs, scope, structure, contract type signatures, and available verification methods.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

How to divide steps and how to verify each step.

## Deferred to later stages

Code.

## Irreversible items

None.

## Artifacts

A list showing each step's purpose, completion conditions, grounds, and scope. Pass the list unchanged to later-stage briefs as previous stage output, following `stages/README.md#work-inputs-and-submission`. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Show each step's purpose, completion conditions, grounds, scope, and verification method. Each step can be verified independently and is small enough for a human to read in one pass.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
