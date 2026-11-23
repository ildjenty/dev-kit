## Purpose

Judge the selected topic's decision proposal independently. You did not write it; verify the requests, evidence and agreed records yourself. The loop and document phase consume your assessment; an invented owner choice or incomplete answer would otherwise become a downstream constraint.

## Inputs

Judge the topic's decisions independently. Read `$ARTIFACTS_DIR/refine/input.json`,
`$ARTIFACTS_DIR/refine/draft.json`, `$ARTIFACTS_DIR/refine/check.json` and the input's
`judge_materials`: named criteria, common rules, authority and numbered principles.
Read all agreed decisions in full. Use Read, Grep and Glob to verify evidence.
The machine owns schema, dependency, reference and checkout-scope checks.

Apply `review/purpose.md` Topic proposals. Return exactly one criterion for each aspect
`coverage`, `additions`, `open_points`, `consistency` and `sufficiency`.

Use the input's common rules and criteria for verdicts and findings. Write in its `language`.
Return `{ "criteria": [{"aspect":"...","verdict":"pass|fail|not_assessed","checked":"...",
"not_checked":"..."}], "findings": [{"aspect":"...","item":"...","blocking":true,
"needs_decision":false,"failure_scenario":"...","evidence":"...","fix":"...","mechanical":false}] }`.
Write no files.

Return every property in the output schema. Use `false` for `mechanical` when no mechanical check is proposed. Use empty strings or empty arrays for inapplicable text or lists.
