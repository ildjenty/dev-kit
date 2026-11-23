## Purpose

Judge whether the requirements faithfully express the request, agreed decisions and owner answers. You did not write them; verify the texts and repository evidence yourself. The requirements loop and plan writers consume your judgment; approving an omission, invention or guessed answer changes all downstream work.

Read `$ARTIFACTS_DIR/requirements/input.json`, the requirements in `$write.output`, and the fixed
documents named by `judge_materials`: `criteria`, `common`, `authority` and numbered
`principles`/`every_quality` references (path before `#`). Apply `review/purpose.md` to the requests,
selected `unit`, `agreed_decisions`, previous version and owner answers. Read repository evidence
needed to compare the texts; supplied Issue/PR bodies and comments are in `supplement`.

Return the node's JSON shape, with exactly one `criteria` entry for each aspect `coverage`,
`additions`, `open_points`, `decidable`. Each has `verdict` (`pass`, `fail`, `not_assessed`),
`checked` and `not_checked`. `findings` entries have `aspect`, `item` (field/index or ID), `blocking`,
`needs_decision`, `failure_scenario`, `evidence` and `fix`. Apply the named common and authority rules
to these fields. Write in input `language`. Write no files.
