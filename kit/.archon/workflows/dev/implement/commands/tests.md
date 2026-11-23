## Purpose

Write one unit's tests before another session implements its bodies. You did not write the requirements or plan; verify their completion conditions yourself. The implement writer and configured checks consume these tests, so expectations copied from imagined implementation can let both sessions make the same mistake.

Read `$ARTIFACTS_DIR/implement/input.json` and the fixed standards named by `test_materials`:
`criteria`, `common`, `authority`, numbered `principles` and `every_quality` references (path before
`#`). Apply the named testing standards.

Read `$ARTIFACTS_DIR/implement/unit.json`, `requirements.json` and `plan.json`. The unit supplies
completion criteria, planned tests and later units; the plan supplies `kept_behaviors`.
Choose expected values from this unit's completion-condition text, never from implementation code.
Do not read a body to decide an expectation. Test only this unit, not `later_units`. Preserve existing
tests of `kept_behaviors`; do not weaken, skip or delete them. The new tests should fail against the
skeleton now. You need not run them before handing off; configured checks run after implementation.
Leave committing to the workflow.

The checkout contains the skeleton and previously committed units. Write the tests of this unit
whose planned method is `test`, using its file and test name. Methods `screen_record` and
`owner_check` produce no test here. Write only test files and their fixtures; another session writes
the implementation. The machine runs checks and commits. Use configured check commands if testing
while writing.

Return the node's JSON shape: `tests` is the list of `<file>: <name>` entries written.
Use input `language` for human-readable text.
