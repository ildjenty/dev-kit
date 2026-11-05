# Verification

## Inputs

The completion condition IDs assigned to this instance's checks; the diff and verification results from implement; the user's entry points (commands, screens, APIs, or other public interfaces through which the change is used); and available verification methods.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

For each assigned completion condition, the usage scenario that shows the condition holds for a user, and whether an end-to-end run through the user's entry point confirms it.

Fix human demo feedback within the approved design and completion conditions in a correction round. For feedback changing either, use `confirmation-required` to recommend back or replanning; do not absorb it into a correction.

## Deferred to later stages

Lessons and recurrence prevention in retrospective, when adopted by the plan.

## Irreversible items

None.

## Artifacts

For each assigned completion condition ID, the usage scenario, the end-to-end commands or steps run, and the observed result. Fixes to defects found by the end-to-end run, with the end-to-end script and its registration where created. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Run each assigned completion condition's usage scenario through the user's entry point, from input to observable result, and report what was observed. Unit tests and calls to internal functions do not replace this run. Mark scenarios that could not be run as unconfirmed with the reason; do not report them as confirmed. An acceptance instance uses this same definition with all completion condition IDs assigned to its checks; the final increment demo may serve as acceptance when it covers them all.

When a required `command:<name>` is unregistered in the round's frozen `devkit.toml`, its check returns an auto-fixable `FAIL`. Create the verification method and register it in `[commands.<name>]`, including its required deadline, following `stages/implement.md`. Registration is a loop edit and is used from the next round's snapshot. Devkit asks the human to confirm loop edits itself, so register the command and claim completion; do not stop to request a separate confirmation for it. For `command:e2e`, create a script that exercises the user's entry point end to end for the assigned completion conditions' usage scenarios and checks the observed results; the `behavior` check assesses whether each scenario is confirmed by that run. If a command changes the tree, ignore its generated output before rerunning verification.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
