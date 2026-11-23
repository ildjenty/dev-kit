## Purpose

Decide the objections to this pass's findings independently. You neither made the findings nor wrote the fix; verify both sides yourself. The repair loop consumes your decisions: accepting a false objection leaves a defect, while rejecting a sound one forces a damaging repair.

## Fixed standards

Read `$ARTIFACTS_DIR/fix/input.json` and its `dispute_materials`: read the named `criteria`, `common`, `authority` and numbered `principles`/`every_quality` references (document path before `#`). These are the fixed standards for this step.

## Input

- The input: `$ARTIFACTS_DIR/fix/input.json`. It names `language`, `standards`, the fixed
  `criteria_folder`, and configured checks (`name`, `command`, optional `paths`). Read all standard
  documents named below relative to that criteria folder. Write owner-facing text in the language
  named by `language` in your input.

- The objections of this pass: `$ARTIFACTS_DIR/fix/objections/<round>.json`, where
  `$ARTIFACTS_DIR/fix/state.json` gives `round`. Each objection holds the finding (`finding`, the
  `judge` that made it, `text`, the judge's `finding_evidence`) and the objection's `evidence`.
- The requirements, or `null` when the request has none (a review-only request):
  `$ARTIFACTS_DIR/fix/requirements.json`; the plan, or `null`: `$ARTIFACTS_DIR/fix/plan.json`.
- The criteria, as fixed for this request: `$ARTIFACTS_DIR/criteria/`. When two findings, or a finding
  and the requirements when there are any, ask for opposite things, decide by the conflict-resolution section of `README.md`
  in that copy.
- The repository with the fix as it is now. Use Read, Grep and Glob to inspect it without changing files.

## How to decide

- Apply the named common rules to the objection's evidence; inspect its referenced files, lines
  and recorded command results with the available read-only tools.
- Accept an objection when what you observed shows the finding's failure cannot happen with the code
  as it is, the criterion the finding cites does not apply to it, or fixing it would break a
  completion condition (when there are requirements) or a decision the owner made.
- Reject an objection when the evidence does not show that, or when it argues only that fixing the
  finding is hard.

## Output

`decisions`: one per objection of this pass, with `finding` (its id), `decision`, `reason` (in
the language named by `language` in your input) and `observations` (what you read and verified, with file and line or recorded command output).

Each `finding` reference includes its original judge run (`<judge-run>/<judge>/<finding-id>`). Return it unchanged so an acceptance applies only to the finding under objection.
