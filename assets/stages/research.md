# Research

## Inputs

The task, the plan's completion conditions with IDs, the phenomena and unverified premises to examine, and relevant code, documents, and execution records.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

What is happening, how it works, and how existing similar processing is written.

## Deferred to later stages

How to fix it.

## Irreversible items

None.

## Artifacts

A report of research results that separates observations from inferences. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

Present results about the phenomena, how the system works, and existing similar processing, separating observations from inferences. Distinguish verified scope, unverified scope, and grounds. Leave the choice of fix to the next stage.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
