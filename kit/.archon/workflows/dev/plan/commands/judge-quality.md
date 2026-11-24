## Purpose

Judge one quality's decision rows and skeleton before function bodies are written. You did not write the plan; verify it yourself. The planning loop consumes your findings before implementation, when a mistaken placement or contract can still be corrected cheaply. Do not substitute a review of unwritten bodies for this decision review.

Read `$INPUTS.materials/$INPUTS.quality.json`. It contains `quality`, `language`, `criteria_folder`, `criteria`, `principles`, `every_quality`, `common`, `authority`, `requirements`, `owner_answers`, your `decisions` and `changes`, `skeleton` (`base`, `commit`, `diff`), `facts` for structure and `previous_findings`.

Read the documents named by `criteria`, `common` and `authority`. Read the numbered sections in `principles` and `every_quality` using the document path before `#`. Apply Decisions in your quality's criteria to your rows and the skeleton, including choices of this quality absent from its rows. Verify author evidence claims against their sources. Use the fixed skeleton and supplied before/after facts for structure.

The checkout is the skeleton under judgment. Compare its supplied diff and named evidence with
what you read yourself. Judge only what rows, files, exports and signatures settle; do not report
missing implementation or conditions that require unwritten bodies. The machine already checked
schema and coverage references; judge the semantic evidence in your assigned concern.

Return the node's JSON shape: `verdict`, `checked`, `not_checked` and `findings`, each with `row` (empty for a missing row), `blocking`, `needs_decision`, `deferred`, `mechanical`, `failure_scenario`, `evidence` and `fix`. For an owner finding, `fix` is the complete decision question under `authority`. Write human-readable text in `language`. Write no files.

For structure, read `facts.locality` (`outside_plan`, top-level and second-level folder counts, and ancestor siblings), and apply the path-of-change and deferred-smell criteria in `review/structure.md`.

Return every property in the output schema. Use `false` for `mechanical` when no mechanical check is proposed and for `deferred` when the finding is on the change’s path. Use empty strings or empty arrays for inapplicable text or lists.
