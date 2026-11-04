# Retrospective

## Inputs

The task and human-approved completion conditions with IDs; the work-history file referenced in the brief, fixed when the instruction was issued; and the norms inventory. Read that file for all judgments and findings, including nonblocking findings, the sequence of `FAIL` and `NOT ASSESSED` results, round counts per stage run, human questions and answers including rejections, corrections and replanning instructions, and human-discovered misses recorded as lessons with `devkit note`. Use `requirements.md`, relevant `structure.json` records, and `rules.md` together with outputs from adopted preceding stages.

Common inputs are the frozen norms, plan limits, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

For each problem or miss, identify where it should have been caught and where to prevent recurrence. Follow `[lessons] destinations` in `devkit.toml`; the default priority is machine checks (hooks, lint, types, and verification commands), plan templates, review documents in `principles/review/`, principles, then skill procedures. Consider unenforced rules and recurring violations for machine enforcement. Assess the failure prevented, execution and maintenance costs, and false positives before adding a mechanism.

Check whether the norms serve their purpose: which principles are used by which review documents, check criteria, and templates, and whether any mechanism is unused or costs more than it contributes. Focus on problems, misses, gaps, and burdens exposed by this task; inventory all norms only when those findings show a need.

Reconcile the approved completion conditions with `requirements.md`. Reflect repository requirements not already recorded by purpose, including when purpose is absent from the plan.

## Deferred to later stages

Record reasons for improvements that are rejected or deferred.

## Irreversible items

Shared difficult-to-change decisions belong in `rules.md` with enforcement and grounds. Norm edits follow the loop-edit confirmation policy in every stage, before their first use and before the completion commit for edits made in the final round.

## Artifacts

A record connecting each problem or miss to the point that should have caught it, its prevention destination, and the adopted change or reason for no change. Apply improvements to repository-owned checks, templates, review documents, principles, or skills. Record lessons with `devkit note`.

Updated `requirements.md` where needed, and any enduring structural or shared decisions in `structure.json` or `rules.md`, following `stages/README.md#decisions-for-later-work`. Runtime records remain untracked and do not replace these repository records.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Account for the task's findings, failed or unavailable assessments, human corrections, and discovered misses, including nonblocking findings. Explain where each problem should have been caught, apply justified prevention measures or record reasons for not applying them, and assess the relevant norms' usefulness and burden. Ensure repository requirements reflect the human-approved conditions even if purpose was omitted.

Changes made in this stage, including changes to tests and verification methods, are verified by this stage's checks in the plan. If a required verification command is unregistered, create the verification method and register it in `devkit.toml`, following `stages/implement.md`; devkit asks the human to confirm the loop edit itself, and the registration takes effect from the next round's snapshot.

All required checks valid for the returned target and norms are `PASS`, and completion is finalized following `stages/README.md#completion-finalization`.
