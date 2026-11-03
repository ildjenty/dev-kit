# Prototype

## Inputs

The plan's completion conditions with IDs, unverified premises to test, observations needed for decisions, and experimental conditions.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Whether unverified premises are true.

## Deferred to later stages

Production code.

## Irreversible items

None.

## Artifacts

Experimental results and decisions. Record decisions with `devkit note` (`docs/architecture/data.md#%E6%B1%BA%E5%AE%9A%E3%81%A8%E8%A8%BC%E6%8B%A0-decision--evidence`). Keep prototypes separate from production code. Before running `devkit submit` (`docs/architecture/flow.md#%E4%BD%9C%E6%A5%AD%E7%B5%90%E6%9E%9C%E3%81%AE%E6%8F%90%E5%87%BA`, the command for submitting work results), delete them or put them in an untracked, ignored location so that the completion commit (`docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6`) excludes them.

## Completion conditions

Show experimental results and decisions about the premises, recording decisions with devkit note. Keep prototypes separate from production code, and delete them before devkit submit or place them in an untracked, ignored location so the completion commit excludes them.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6` for finalization responsibilities.
