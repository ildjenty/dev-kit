## Purpose

Implement one release unit against its completion conditions and independently written tests. You did not write those tests or the passed plan; verify them against the requirements yourself. Checks and the workflow consume your code and commit message before the next unit; coding later units or adjusting tests hides whether this unit is complete.

Read `$ARTIFACTS_DIR/implement/input.json` and the fixed standards named by `writer_materials`:
`criteria`, `common`, `authority`, numbered `principles` and `every_quality` references (path before
`#`). Apply these standards to the implementation.

Read `$ARTIFACTS_DIR/implement/unit.json`, `requirements.json` and `plan.json`. The unit names its
completion criteria, planned tests and later units. The checkout contains the skeleton, prior
committed units and tests written by a separate session. Write this unit's code in the checkout;
leave tests to the test-writing step and committing to the machine. For later passes read
`$ARTIFACTS_DIR/checks/round`, the corresponding `<round>.jsonl` and named logs, and
`$ARTIFACTS_DIR/implement/decision-errors.json` when present.

Address input `returned` premises and observations first.
Start from the skeleton and write bodies for this unit only; do not implement `later_units`.
Keep the skeleton's files, exports and signatures unless a body proves a change is needed; record
such a change as a decision below. A change outside the allowed update must return to plan.
Never change this unit's tests to make them pass. If a test misreads a completion condition, record
the evidence in `uncertain` and leave the test for correction by its responsible step.
On a later pass fix the cause shown by the check logs, not just the symptom. Correct the specific
rows named by `decision-errors.json`, retaining other valid decisions and their IDs.

Record only allowed plan updates in `$ARTIFACTS_DIR/implement/decisions.json` as `{ "rows": [...] }`,
using the plan row schema. Retain rows from prior passes and IDs for existing rows. New rows must be
provisional and use a new ID; a kept or settled row retains its ID. A `later` row may be settled provisionally while preserving its question, candidates,
principles, quality and touches. Other plan changes return to plan. Evidence uses `source: author`
and a verification location. Use the input's configured check commands when testing during writing.

Return the node's JSON shape in input `language`:

- `owner_decisions`: `{ "row": "<existing or new row ID>", "question": "..." }` for choices
  reserved by the named authority document. Leave dependent code unwritten; the machine returns
  `back` to plan, which asks the question.
- `uncertain`: unresolved observations.
- `commit_message`: one line with a prefix from fixed `git.md`, followed by the unit's result derived
  from the requirements' goal and this unit's completion conditions. Do not include unit or condition
  IDs or `ALL`; the machine adds provenance trailers. Leave committing to the workflow.
