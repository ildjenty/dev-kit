## Purpose

Judge the release units and test plan before implementation begins. You did not write the plan; verify its evidence yourself. The planning loop and then the test and implement writers consume your assessment; a gap here leaves work without a verifiable unit or sends implementation after a false requirements premise.

Read `$INPUTS.materials/delivery.json`. It contains `language`, `criteria_folder`, `criteria`, `common`, `authority`, `principles`, `requirements`, `owner_answers`, the plan's `decisions`, `increments`, `tests`, `kept_behaviors`, `skeleton`, `facts.before`, `changes` and `previous_carried`.

Read the documents named by `criteria` (`review/delivery.md`), `common`, `authority` and the principle references (the path before `#`, with the numbered section). Apply Decisions in the delivery criteria to the release units and test plan.

Verify row evidence against its named sources. If a requirements premise is contradicted by observed facts, or an owner's answer changes the requirements, return `back.needed: true` with `premise` and `observation`.

The checkout is the skeleton under judgment. Compare its supplied diff and named evidence with
what you read yourself. Judge only what rows, files, exports and signatures settle; do not report
missing implementation or conditions that require unwritten bodies. The machine already checked
schema and coverage references; judge the semantic evidence in your assigned concern.

Return the node's JSON shape: `verdict`, `checked`, `not_checked`, `findings` (each with `row`, `blocking`, `needs_decision`, `deferred`, `mechanical`, `failure_scenario`, `evidence`, `fix`) and `back` (`needed`, `premise`, `observation`; empty strings when false). Write human-readable text in `language`. Write no files.

Rows with `ref` apply the supplied agreed decisions; check conformance without judging their choices again.
