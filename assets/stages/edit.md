# Document editing

## Inputs

The task, human-approved completion conditions with IDs, and the documents to edit.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Wording, examples, and organization needed to meet the agreed documentation requirements while preserving the public contract.

## Deferred to later stages

Lessons and recurrence prevention in retrospective.

## Irreversible items

None.

## Artifacts

The documentation diff and evidence corresponding to completion condition IDs. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

The documents satisfy the agreed completion conditions. Ask the human about undecidable requirements instead of guessing.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
