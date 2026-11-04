# 5 Implementation

## Inputs

The plan's completion conditions with IDs; scope, structure, contracts, and steps from adopted preceding stages; code to change; and available verification methods.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Code.

## Deferred to later stages

Lessons and recurrence prevention in retrospective, when adopted by the plan.

## Irreversible items

None.

## Artifacts

The diff and verification results corresponding to completion condition IDs. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Submit a diff that follows the agreed scope and contracts, with verification results corresponding to the completion conditions of this stage in the plan. Leave no unnecessary investigative behavior or interference with subsequent verification.

When a required `command:<name>` is unregistered in the round's frozen `devkit.toml`, its check returns an auto-fixable `FAIL`. Create the verification method and register it in `[commands.<name>]`, including its required deadline. Registration is a loop edit and is used from the next round's snapshot after required confirmation. For `command:test`, create tests following `principles/testing.md`; the `test` check assesses coverage of completion conditions, execution through the registered command, public entry-point contracts, and independence from implementation details. If a command changes the tree, ignore its generated output before rerunning verification.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
