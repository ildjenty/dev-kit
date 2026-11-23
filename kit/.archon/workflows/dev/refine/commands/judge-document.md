## Purpose

Judge whether the documents and approval text are understandable to their readers. You did not write them; verify their wording yourself. The loop and owner consume your assessment before approval; unclear wording can turn a correct decision into the wrong implementation.

## Inputs

Judge the documents independently. Read `$ARTIFACTS_DIR/refine/input.json`,
`$ARTIFACTS_DIR/refine/decided.json`, `$ARTIFACTS_DIR/refine/draft.json`,
`$ARTIFACTS_DIR/refine/check.json`, `$ARTIFACTS_DIR/refine/approval.txt`, the edited documents,
and the input's `judge_materials`: named criteria, common rules, authority and numbered principles.
Read the affected documents and units, including reasons for keeping them.
The machine owns reference, frozen-decision, unit-area and checkout-scope checks. The reflect step owns decision reflection.

Apply the named wording criteria to documents and approval text. Return exactly one
criterion for aspect `wording`.

Use the input's common rules and criteria for verdicts and findings. Write in its `language`.
Return `{ "criteria": [{"aspect":"...","verdict":"pass|fail|not_assessed","checked":"...",
"not_checked":"..."}], "findings": [{"aspect":"...","item":"...","blocking":true,
"needs_decision":false,"failure_scenario":"...","evidence":"...","fix":"...","mechanical":false}] }`.
Write no files.
