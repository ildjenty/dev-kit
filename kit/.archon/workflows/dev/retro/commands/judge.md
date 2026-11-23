## Purpose

Judge the retrospective proposal independently against its backlog evidence. You did not write it or the original findings; verify both yourself. The loop and owner use this assessment before corrections become active; an unsupported correction can weaken future checks or add a mechanism that does not prevent the observed failure.

Read `$ARTIFACTS_DIR/retro/input.json`, `$ARTIFACTS_DIR/retro/draft.json`,
`$ARTIFACTS_DIR/retro/check.json`, `$ARTIFACTS_DIR/retro/approval.txt`, and
`$ARTIFACTS_DIR/retro/judgment.schema.json`. Inspect the proposed config and standard edits in the checkout.
Use only the fixed criteria copy named by input's `criteria` and `judge_materials`, including
`harness.md`, `review/harness.md`, `review/README.md` and `authority.md`.

Apply the named criteria documents and return the aspects `traceability`, `evidence` and `mechanism`.

Return an object matching the supplied judgment schema and node output format: `criteria` contains
exactly one row per aspect, with `aspect`, `verdict` (pass, fail, not_assessed), `checked` and
`not_checked`; `findings` contains `aspect`, `item`, `blocking`, `needs_decision`,
`failure_scenario`, `evidence`, `fix` and optional `mechanical`. Write owner-facing text in input `language`.

Return every property in the output schema. Use `false` for `mechanical` when no mechanical check is proposed. Use empty strings or empty arrays for inapplicable text or lists.
