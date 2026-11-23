## Purpose

Complete the remaining decision rows, release units and verification plan inside the judged structure. You did not author the requirements or necessarily the structure; verify them yourself. Quality and delivery judges, then separate test and implement writers, consume this output; an incomplete handoff can omit a condition or build units in the wrong order.

Read `$ARTIFACTS_DIR/plan/input.json`, `$ARTIFACTS_DIR/plan/structure.json`, `$ARTIFACTS_DIR/plan/facts-before.json` and, on later rounds, `$ARTIFACTS_DIR/plan/findings.json`. Read the fixed standards named by input `writer_materials`, including the relevant quality's Decisions, the common review rules and authority document.

Address `returned` premises and observations first.
For superseded decisions in `returned`, change only items that rest on the old decision; keep everything else as it is. When a successor changes nothing for an item, only move its citation to the successor.
On a later round correct the machine errors,
blocking findings that need no owner decision and missing evidence you can supply; keep what holds.
Verify claims against their cited files or library definitions before relying on them.

Complete `$ARTIFACTS_DIR/plan/draft.json` using `$ARTIFACTS_DIR/plan/draft.schema.json`. Preserve the structure rows from `structure.json` unchanged. Add other qualities' rows, applying agreed decisions by `ref` and including pending owner rows with their existing IDs. Each row carries its facts as `evidence`, `source: machine` or `source: author`, with `text` naming where to verify them. Retain IDs of existing rows and apply only answers bound to their requirements version and row content.

Write ordered `increments` as `{ "id": "U1", "summary": "...", "criteria": ["C1"] }`,
retaining IDs of kept units. Use the named delivery Decisions for ordering and coverage.
For `tests`, name each completion condition's `criterion` and its chosen `method`: `test` has the
exact `file` and test `name`; `screen_record` and `owner_check` retain that chosen verification.
Record `kept_behaviors` and how each is checked, separately from behavior the request changes. You may change skeleton signatures inside the judged structure. Keep new bodies as loud failures,
move existing bodies unchanged when required, and add no new logic, tests or comments.
Change nothing the requirements do not need, and leave committing to the workflow. Add, remove or move no file or folder relative to `structure.json`'s `skeleton_commit`. If a structure change is necessary, describe it in `uncertain`; the machine ends the run with `stop` when it detects the change, and plan must start again.

Return the node's JSON object: `prototype`, `structure_question`, `provisional` and `uncertain`. Preserve the structure question from the structure phase; use an empty `prototype` unless separate prototype evidence is needed. Write owner-facing texts in input `language`.
