# 1 Scope

## Inputs

The task, the plan's completion conditions with IDs, the relationship between purpose and completion conditions, and existing external contracts and data retention conditions.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

What is included, what is excluded, and what is externally visible.

## Deferred to later stages

Internal structure.

## Irreversible items

Schemas, identifiers, contract types, external publication, and data retention identified as difficult to change.

## Artifacts

An explicit scope and list of irreversible items. Record shared difficult-to-change decisions with enforcement and grounds in `rules.md`, and task-specific irreversible decisions in the plan (`schemas/plan.json`). Later-stage briefs include irreversible items from both, following `stages/README.md#work-inputs-and-submission`. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Make inclusions, exclusions, and externally visible behavior explicit. Show the list of difficult-to-change decisions, unverified matters needed to decide them, and when to finalize them. Do not begin dependent implementation, publication, or storage while matters affecting the choice remain unverified.

All required checks valid for the returned target and norms are `PASS`, the detection requirements in `examples/README.md` are satisfied, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
