## Purpose

Judge the retrospective proposal commit independently against its backlog evidence. You did not write it or the original findings; verify both yourself. The loop and owner use this assessment before approval; an unsupported correction can weaken future checks or add a mechanism that does not prevent the observed failure.

Read `$ARTIFACTS_DIR/retro/input.json`, `$ARTIFACTS_DIR/retro/draft.json`,
`$ARTIFACTS_DIR/retro/check.json`, `$ARTIFACTS_DIR/retro/approval.txt`, and
`$ARTIFACTS_DIR/retro/judgment.schema.json` and `$ARTIFACTS_DIR/retro/baseline.json`.
The machine has committed the proposal at baseline's `commit`; inspect the real config, standards,
units, cases and consumed backlog deletions in that committed checkout. Do not edit it.
Use only the fixed criteria copy named by input's `criteria` and `judge_materials`, including
`harness.md`, `review/harness.md`, `review/README.md` and `authority.md`.

For every escape, check the trace's evidence of the entering change, the stage that should have
stopped it and why it missed. If the entering change could not be read, require that limitation
explicitly rather than a guessed history. Check that routing misses correct the relevant routing
sentence in `concerns.json`. For each used escape the judge should have caught, inspect the new
evaluation case: it must reproduce the entering defect with minimal code and meaningful expected
findings, not the fixing change. Genuine gaps in the request must be justified. These files are
proposed edits, not criteria that override the fixed standards.

Under `evidence`, check `pr_title` and `pr_summary` as text for this PR's reviewer in the language
named by `language` in your input: the short title names the change and the few-sentence summary
explains what merging puts on the base and why. Neither repurposes approval instructions, versions
or internal history. The machine appends these exact fields to the approval question and uses them
as approved, followed by lists of approved changes, escape traces, cases and Issues.

Apply the named criteria documents and return the aspects `traceability`, `evidence` and `mechanism`.

Return an object matching the supplied judgment schema and node output format: `criteria` contains
exactly one row per aspect, with `aspect`, `verdict` (pass, fail, not_assessed), `checked` and
`not_checked`; `findings` contains `aspect`, `item`, `blocking`, `needs_decision`,
`failure_scenario`, `evidence`, `fix` and optional `mechanical`. Write owner-facing text in input `language`.

Return every property in the output schema. Use `false` for `mechanical` when no mechanical check is proposed. Use empty strings or empty arrays for inapplicable text or lists.

Each unit Issue draft must fit one PR, have observable completion conditions and include all delivery constraints without requiring decision records. Check the blocked-by order among drafts.
