# Initialization

## Inputs

The task, the plan's completion conditions with IDs, repository layout and existing startup and test methods, and settings for the host in use.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in rules.md and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required command: check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

How to start the system, the first tests to pass, how to show progress, and how to choose the next work. Use `devkit init` to install the kit and `devkit doctor` to verify the installed kit and host settings; follow `skills/devkit/SKILL.md` for host trust and session startup.

## Deferred to later stages

Implementation.

## Irreversible items

None.

## Artifacts

A working environment.

## Completion conditions

Show how to start the system, the first tests to pass, how to show progress, and how to choose the next work, with the environment working. Install the kit with `devkit init` and verify it with `devkit doctor`.

All required checks valid for the returned target and norms are `PASS`, detection tests have passed for required aspects handled by judges and their assigned runners, and devkit has successfully checked the target and run `git commit`. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
