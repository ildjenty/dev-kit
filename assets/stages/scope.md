# 1 Scope

## Inputs

The task, the plan's completion conditions with IDs, the relationship between purpose and completion conditions, and existing external contracts and data retention conditions.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

What is included, what is excluded, and what is externally visible.

## Deferred to later stages

Internal structure.

## Irreversible items

Schemas, identifiers, contract types, external publication, and data retention identified as difficult to change.

## Artifacts

An explicit scope and list of irreversible items. Record shared difficult-to-change decisions with enforcement and grounds in `.devkit/rules.md`, and task-specific irreversible decisions in the plan (`schemas/plan.json`). Later-stage briefs include irreversible items from both, following `stages/README.md#work-inputs-and-submission`.

## Completion conditions

Make inclusions, exclusions, and externally visible behavior explicit. Show the list of difficult-to-change decisions, unverified matters needed to decide them, and when to finalize them. Do not begin dependent implementation, publication, or storage while matters affecting the choice remain unverified.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
