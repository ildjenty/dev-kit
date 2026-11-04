# 2 Structure

## Inputs

The plan's completion conditions with IDs, scope, the list of difficult-to-change decisions, existing structure and placement guides, and similar processing.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Modules, responsibilities, placement, and dependency direction.

## Deferred to later stages

Function internals.

## Irreversible items

Placement choices identified as difficult to change.

## Artifacts

A new version of `.devkit/structure.json` (modules, responsibilities, placement, and dependency direction; versioned by content hash, with history in git and approval through PR review; devkit generates diagrams), existing locations whose style to follow, rejected alternatives and reasons, and unverified premises. Include recovery methods if effects cannot be reverted with git or require a special recovery order.

## Completion conditions

Reflect modules, responsibilities, placement, and dependency direction in the structure record. Show existing locations to follow, rejected alternatives and reasons, and unverified premises. Provide recovery methods when needed. Finalize difficult-to-change placement only after resolving unverified matters affecting the choice.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
