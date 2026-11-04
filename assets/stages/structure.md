# 2 Structure

## Inputs

The plan's completion conditions with IDs, scope, the list of difficult-to-change decisions, existing structure and placement guides, and similar processing.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Modules, responsibilities, placement, and dependency direction.

## Deferred to later stages

Function internals.

## Irreversible items

Placement choices identified as difficult to change.

## Artifacts

Updated `structure.json` following the bundled `schemas/structure.json` at the working-tree path shown in the brief. Record modules with `id`, `name`, `responsibility`, and `path`, and dependencies with `from` and `to` module IDs. Module IDs must be unique and both ends of each dependency must name recorded modules. Include existing locations whose style to follow, rejected alternatives and reasons, and unverified premises in the stage output. Include recovery methods if effects cannot be reverted with git or require a special recovery order. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

Reflect modules, responsibilities, placement, and dependency direction in the structure record using `schemas/structure.json`. Show existing locations to follow, rejected alternatives and reasons, and unverified premises. Provide recovery methods when needed. Finalize difficult-to-change placement only after resolving unverified matters affecting the choice.

All required checks valid for the returned target and norms are `PASS`, the detection requirements in `examples/README.md` are satisfied, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
