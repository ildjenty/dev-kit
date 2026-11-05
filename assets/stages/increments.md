# 3 Increments

## Inputs

The plan's completion conditions with IDs, approved scope and architecture, module structure, and available startup and verification methods.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

Release increments, their order, completion condition IDs, scope, and verification methods. Make the first increment a thin path from the user interface through to data, including startup and testing. Split the rest by completion conditions so each fits within one implement instance's limits. One increment is enough for a small task.

## Deferred to later stages

Each increment's detailed design and implementation.

## Irreversible items

None.

## Artifacts

An ordered increment list with IDs, purpose, assigned completion condition IDs, scope, size grounds, and verification method. Include explicit conditions for verifying the skeleton.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Cover the agreed completion conditions with increments that can be demonstrated through the user's entry point. State how startup, testing, and the skeleton will be verified. Finish with `done: "replan"` so the planner builds the increment instances and the human reviews their division and order.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
