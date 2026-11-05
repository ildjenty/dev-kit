# 1 Scope

## Inputs

The plan's completion conditions with IDs, agreed requirements and usage scenarios, and existing external contracts and data retention conditions.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Screens, user flows, inputs and outputs, the data model, and exclusions. When UI cannot be judged from prose, have the planner insert a prototype before this stage.

## Deferred to later stages

Technology choices and overall module structure to architecture; increment details to design.

## Irreversible items

Data models, identifiers, external contracts, publication, and retention choices that are difficult to change.

## Artifacts

External design and scope, with screens, flows, inputs and outputs, the data model, exclusions, and difficult-to-change decisions. Preserve decisions needed by future tasks following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Make the external design concrete enough for the human to judge at the scope gate. Propose its human decisions for that gate as provisional, following `stages/README.md#decisions-for-later-work`. Identify unresolved matters affecting difficult-to-change choices before dependent work begins.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
