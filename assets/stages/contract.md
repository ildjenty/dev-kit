# 3 Contract

## Inputs

The plan's completion conditions with IDs, scope, structure records, existing public and internal contracts, and the list of difficult-to-change decisions.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Interfaces, data formats, and failure types.

## Deferred to later stages

Implementation approach.

## Irreversible items

Public and internal contracts identified as difficult to change.

## Artifacts

Type signatures for interfaces, data formats, and failure types. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

Express interfaces, data formats, and failure types as type signatures so that guarantees against accepting invalid states and public failure responses can be assessed. Finalize difficult-to-change contracts only after resolving unverified matters that affect the choice.

All required checks valid for the returned target and norms are `PASS`, the detection requirements in `examples/README.md` are satisfied, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
