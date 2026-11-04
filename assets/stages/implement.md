# 5 Implementation

## Inputs

The plan's completion conditions with IDs; scope, structure, contracts, and steps from adopted preceding stages; code to change; and available verification methods.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Code.

## Deferred to later stages

None.

## Irreversible items

None.

## Artifacts

The diff.

## Completion conditions

Submit a diff that follows the agreed scope and contracts, with verification results corresponding to the completion conditions of this stage in the plan. Leave no unnecessary investigative behavior or interference with subsequent verification.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
