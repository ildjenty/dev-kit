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

An explicit scope and list of irreversible items. Follow rules and enforcement (`docs/architecture/data.md#%E8%A6%8F%E5%89%87%E3%81%A8%E5%BC%B7%E5%88%B6%E6%89%8B%E6%AE%B5-rules`) and the plan (`docs/architecture/data.md#%E8%A8%88%E7%94%BB-plan`) for storage. Follow rules and enforcement (`docs/architecture/data.md#%E8%A6%8F%E5%89%87%E3%81%A8%E5%BC%B7%E5%88%B6%E6%89%8B%E6%AE%B5-rules`) for the scope passed to later-stage briefs (`docs/architecture/modules.md#brief`, requests assembled from inputs).

## Completion conditions

Make inclusions, exclusions, and externally visible behavior explicit. Show the list of difficult-to-change decisions, unverified matters needed to decide them, and when to finalize them. Do not begin dependent implementation, publication, or storage while matters affecting the choice remain unverified.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6` for finalization responsibilities.
