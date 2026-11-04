# Prototype

## Inputs

The plan's completion conditions with IDs, unverified premises to test, observations needed for decisions, and experimental conditions.

Common inputs are `requirements.md`, the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Whether unverified premises are true.

## Deferred to later stages

Production code.

## Irreversible items

None.

## Artifacts

Experimental results and decisions. Agent workers put decisions and evidence in the termination block's `notes` array as `{"kind":"decision|evidence","payload":{...}}`, using `schemas/decision.json` or `schemas/evidence.json` for the payload. The session agent itself uses `devkit note <kind> <file>`. Keep prototypes separate from production code. Before submitting work with `devkit submit <file>` (`stages/README.md#work-inputs-and-submission`), delete them or put them in an untracked, ignored location so the completion commit excludes them.

Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`; runtime notes alone are not enough.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Show experimental results and decisions about the premises, recording decisions in the termination block's notes for agent workers, or with devkit note for the session agent itself. Keep prototypes separate from production code, and delete them before devkit submit or place them in an untracked, ignored location so the completion commit excludes them.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
