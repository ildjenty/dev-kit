# 3 Contract

## Inputs

The plan's completion conditions with IDs, scope, structure records, existing public and internal contracts, and the list of difficult-to-change decisions.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Interfaces, data formats, and failure types.

## Deferred to later stages

Implementation approach.

## Irreversible items

Public and internal contracts identified as difficult to change.

## Artifacts

Type signatures.

## Completion conditions

Express interfaces, data formats, and failure types as type signatures so that guarantees against accepting invalid states and public failure responses can be assessed. Finalize difficult-to-change contracts only after resolving unverified matters that affect the choice.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6` for finalization responsibilities.
