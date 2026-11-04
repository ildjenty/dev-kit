# Handoff

## Inputs

The plan's completion conditions with IDs; the diff accumulated by this work; outputs from adopted preceding stages, including verification results; `templates/pr.md.tmpl`; and the rules for PR descriptions in `rules.md` and the git rules document.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

What reviewers need to know to review the change: summary, change scope, plan summary, decisions, verification, unconfirmed items and risks, and references.

## Deferred to later stages

Pushing, creating the PR, and integration, which the session agent performs after the work completes following the git rules.

## Irreversible items

None. The stage performs no git operations; it writes the description only.

## Artifacts

A PR description for reviewers following the sections of `templates/pr.md.tmpl`, written to a file whose path is reported. Do not create branches, push, or open PRs in this stage.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Write the description so that a reviewer who has not seen this work's conversation can understand what changed, why, how it was verified, and what remains unconfirmed. Base each statement on the diff and the adopted stage outputs; do not claim verification that those outputs do not show.

All required checks valid for the returned target and norms are `PASS`, the detection requirements in `examples/README.md` are satisfied, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
