## Purpose

Judge the assembled pull request body before publication. You did not write it; verify its claims yourself. The publication step and the owner consume your judgment, and the owner decides whether to merge without reading code. Unsupported verification claims or hidden remaining work can let an unsuitable change through.

Read `$ARTIFACTS_DIR/pr/facts.json` and `judge_materials`. Read the named `criteria`, `common`, `authority` and numbered `principles` references (document path before `#`).

# Judge the pull request body

Read `$ARTIFACTS_DIR/pr/facts.json` for `language`, `standards` and `criteria_folder`. Judge by `review/report.md` and the documents it names in that fixed criteria folder, never the checkout's edited criteria.

Read `pr.md`, `pr-title.txt`, `lists.json`, `diff.patch`, `requirements.json` and `judgments.json` in `$ARTIFACTS_DIR/pr/`. Verify body claims against the diff, judgment records and relevant repository evidence yourself.
Name the texts or file locations you compared in observations; unsupported claims are not proof.
Apply the named report criteria. The body must contain generic sections for what changed, how it was checked, what remains (carried findings and not-checked items), decisions, and work after merge. Confirm the machine lists are present, including owner and principle decision rows; do not treat a test-plan entry as proof of execution. The owner decides whether to merge from this body without reading code.

The machine may append `Closes #<number>` to close the request issue on merge. Accept this footer as machine-added publication metadata.

Return `verdict` (`pass`, `fail`, `not_assessed`), `findings` (`id`, `blocking`, `needs_decision`, `text`, `evidence`), and `observations`. Write human-readable output in the language named by `language` in your input.
