# 4 Design

## Inputs

The current increment's scope and completion condition IDs, approved architecture, existing modules and contracts, and the increment list.

Common inputs are `requirements.md`, the authority rules (`authority` source), the frozen norms, plan limits, relevant records from `structure.json`, placement guides for the change targets, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from previous stages. Fix rounds also include previous findings and their IDs, required `command:` check output, and pre-commit output. Use outputs only from stages adopted by the plan; do not require absent stages.

## Decisions

The increment's modules, interfaces, types, data formats, and error handling. Classify each decision against the authority rules as requiring a human decision or delegated. Ask for human decisions with `confirmation-required`, options, and a recommendation.

## Deferred to later stages

Code and tests to implement; demonstration to verification.

## Irreversible items

Changes to approved architecture, data models, public interfaces, or module boundaries requiring a human decision under the authority rules.

## Artifacts

A design document with the decision list, module responsibilities, interface type signatures, data formats, and failure types. Update `structure.json` and shared choices in `rules.md` where needed, following `stages/README.md#decisions-for-later-work`.

## Completion conditions

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Resolve decisions requiring human answers and unverified matters affecting the design. Keep the design within the approved architecture and assigned increment. Record the answered decisions with their question and answer IDs before dependent implementation; cite the scope or architecture gate question ID for decisions approved at that gate.

All required checks valid for the returned target and norms are `PASS`, and devkit has successfully finalized completion at a matching committed target. Follow the plan for selecting and requiring verification; do not add checks from this definition. Follow `stages/README.md#completion-finalization` for finalization responsibilities.
