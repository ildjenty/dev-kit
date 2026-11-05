# 2 Architecture

## Inputs

The plan's completion conditions with IDs, approved external design and scope, existing technology and structure, and research or prototype results relevant to the choices.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Language, runtime, framework, datastore, deployment environment, major libraries, overall modules, responsibilities, placement, and dependency direction. Give alternatives, a recommendation, reasons, and cost and operational effects for each choice. Have the planner insert research or prototype before this stage when an unverified premise affects a choice.

## Deferred to later stages

Release increments and their detailed interfaces, types, and error handling.

## Irreversible items

Technology choices, architecture, module boundaries, and dependency direction identified as difficult to change.

## Artifacts

The decision list and initial `structure.json` following `schemas/structure.json`, with existing locations to follow, rejected alternatives, unverified premises, and recovery methods where needed. Preserve approved shared choices in `rules.md` and the module structure in `structure.json`, following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Resolve premises affecting the choices and present the architecture for the human gate. Propose its human decisions for that gate as provisional, following `stages/README.md#decisions-for-later-work`; record decisions already decided with their grounds, and keep modules and dependencies consistent with the proposals.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
