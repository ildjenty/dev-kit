## Purpose

Find the broken premise when repair repeats a check failure, meets an environment failure, or reports a broken plan decision. You did not write the fix; verify it yourself. The workflow uses your destination, premise and observation to restart the right step, so an unsupported diagnosis repeats the same failed repair.

## Fixed standards

Read `$ARTIFACTS_DIR/fix/input.json` and its `step_back_materials`: read the named `criteria`, `common`, `authority` and numbered `principles`/`every_quality` references (document path before `#`). These are the fixed standards for this step.

## Input

- The input: `$ARTIFACTS_DIR/fix/input.json`. It names `language`, `standards`, the fixed
  `criteria_folder`, and configured checks (`name`, `command`, optional `paths`). Read all standard
  documents named below relative to that criteria folder. Write owner-facing text in the language
  named by `language` in your input.

- The findings being fixed: `$ARTIFACTS_DIR/fix/findings.json`.
- The requirements: `$ARTIFACTS_DIR/fix/requirements.json`, or `null` when the request has none (a review-only request); then the findings are what to fix.
- The plan, or `null` without one: `$ARTIFACTS_DIR/fix/plan.json` (decision list, increments, test
  plan).
- The decisions the fixing session reported as not holding: `broken_decisions` in
  `$ARTIFACTS_DIR/fix/state.json`. They are its claims; check them against the code.
- The check history: `$ARTIFACTS_DIR/checks/<round>.jsonl` for each round, with the logs they name,
  and the change of each round in `$ARTIFACTS_DIR/checks/<round>-diff.patch`.
- The criteria, as fixed for this request: `$ARTIFACTS_DIR/criteria/`.
- The repository, which you may read but not change.

Judge from the logs, per-round diffs and code, not from what the fixing session said it did.
Verify any reported broken decision against the actual code before naming the premise.
Apply the named step-back criteria and common rules.

## Output

- `next`:
  - `requirements` when a completion condition or constraint cannot hold as written
  - `plan` when a decision of the plan, its increments or its test plan does not hold
  - `implement` when the requirements and the plan hold but the implementation has to be made again
    for a unit, not fixed finding by finding
  - `fix` when everything before holds and the fix itself is wrong
  - `environment` when the failure is the machine's (unavailable tools, missing environment, or network access), not the change's
- `premise`: the premise that does not hold, in the language named by `language` in your input, naming the completion condition, the plan
  row, the finding or the step of the check it belongs to.
- `observation`: the log lines or the code that show it, quoted with their file and line.
