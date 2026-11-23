## Purpose

Find the broken premise after repeated implementation check failures or an environment failure. You did not write the implementation; verify it yourself. The workflow routes your diagnosis upstream, where the writer must address it before trying again; a diagnosis based on the implementer's account can repeat the same failed premise.

## Fixed standards

Read `$ARTIFACTS_DIR/implement/input.json` and its `step_back_materials`: read the named `criteria`, `common`, `authority` and numbered `principles`/`every_quality` references (document path before `#`). These are the fixed standards for this step.

## Input

- The input: `$ARTIFACTS_DIR/implement/input.json`. It names `language`, `standards`, the fixed
  `criteria_folder`, and configured checks (`name`, `command`, optional `paths`). Read all standard
  documents named below relative to that criteria folder. Write owner-facing text in the language
  named by `language` in your input.

- The requirements: `$ARTIFACTS_DIR/implement/requirements.json`.
- The passed plan: `$ARTIFACTS_DIR/implement/plan.json` (decision list, increments,
  test plan).
- The check history: `$ARTIFACTS_DIR/checks/<round>.jsonl` for each round, with the logs they name,
  and the change of each round in `$ARTIFACTS_DIR/checks/<round>-diff.patch`.
- The criteria, as fixed for this request: `$ARTIFACTS_DIR/criteria/`.
- The repository, which you may read but not change.

Judge from the logs, per-round diffs and code, not from what the implementation session said it did.
Verify any reported broken decision against the actual code before naming the premise.
Apply the named step-back criteria and common rules.

## Output

- `next`:
  - `requirements` when a completion condition or constraint cannot hold as written
  - `plan` when a decision of the plan, its increments or its test plan does not hold
  - `implement` when the requirements and the plan hold and the implementation is wrong
  - `environment` when the failure is the machine's (unavailable tools, missing environment, or network access), not the change's
- `premise`: the premise that does not hold, in the language named by `language` in your input, naming the completion condition, the plan
  row or the step of the check it belongs to.
- `observation`: the log lines or the code that show it, quoted with their file and line.
