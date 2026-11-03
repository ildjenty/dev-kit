# 4 Steps

## Inputs

The plan's completion conditions with IDs, scope, structure, contract type signatures, and available verification methods.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

How to divide steps and how to verify each step.

## Deferred to later stages

Code.

## Irreversible items

None.

## Artifacts

A list showing each step's purpose, completion conditions, grounds, and scope. Follow the work instruction contract (`docs/architecture/flow.md#%E4%BD%9C%E6%A5%AD%E3%81%AE%E6%8C%87%E7%A4%BA`) for inclusion in later-stage briefs.

## Completion conditions

Show each step's purpose, completion conditions, grounds, scope, and verification method. Each step can be verified independently and is small enough for a human to read in one pass.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6` for finalization responsibilities.
