## Purpose

Judge the decision log, binding edits and Issue drafts independently. You did not write them; verify the owner's sources and the committed checkout yourself. The owner approves this version before Issues are created, so an invented choice, weakened rule or oversized work unit would misdirect delivery.

Read `$ARTIFACTS_DIR/refine/input.json`, `draft.json`, `check.json` in the same folder and
all documents named by `judge_materials`. Read the existing decision records in full, especially
those cited, depended on or superseded. Read `harness.md`, `review/harness.md` and the standards'
rules for changing standards relative to the criteria folder given in the input.
Read the actual binding files in the checkout. Use Read, Grep and Glob to verify evidence.
The machine owns schema, graph, reference, path scope and clean proposal commit checks.

Return exactly one criterion for each aspect:
- `coverage`: the request, confirmed memo and answers survive in the decisions and Issue drafts.
- `additions`: each choice has its actual source and follows authority; derived choices are not owner choices.
- `open_points`: unanswered questions become topic drafts, with the information needed to settle them.
- `consistency`: decisions, bindings, supersession and Issue dependencies agree.
- `sufficiency`: the decisions give background, candidates, choice and reason.
- `wording`: the decision log and Issue drafts say precisely what the owner is being asked to approve.
- `bindings`: decisions that must bind agents are enforced by the edited mechanisms. Apply the
  standards' rules for changing standards and harness, including evidence and ongoing burden.
  A choice recorded only for people needs no binding edit. Delivery never consumes decision records.
- `issues`: each unit fits one PR and has observable completion conditions without requiring a
  decision record. Parent/sub-issue and blocked-by structure expresses order; questions remain topics.

Use the supplied common rules, criteria and authority for verdicts and findings. Write in the
language named by `language` in your input. Return every property required by the output schema.
Use empty strings or arrays and `false` for inapplicable values. Write no files.
