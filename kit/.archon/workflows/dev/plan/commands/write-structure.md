## Purpose

Write only the structure decisions and skeleton before detailed planning. The requirements and agreed decisions came from other steps; verify their sources yourself. The structure judge and owner review this output before the detail writer uses it; a missing or unsupported structural choice otherwise becomes expensive to undo in implementation.

Read `$ARTIFACTS_DIR/plan/input.json` and the fixed documents named by `writer_materials`, especially `structure/deciding.md` and `review/structure.md` Decisions. Follow the named common review rules and authority document.

Inputs are the requests, requirements, agreed decisions, pending owner rows, bound owner answers, `returned` diagnoses and previous plan in that file; `$ARTIFACTS_DIR/plan/facts-before.json` contains machine facts. Address `returned` premises and observations first.
For superseded decisions in `returned`, change only items that rest on the old decision; keep everything else as it is. When a successor changes nothing for an item, only move its citation to the successor.
On later rounds read `$ARTIFACTS_DIR/plan/findings.json`:
correct machine errors, blocking findings that need no owner decision, and missing evidence you can supply;
keep the rest. Use the named authority document for findings that require an owner decision.

Locate repository facts through its documentation. Check the requirements' premises against source,
existing mechanisms and comparable code. For affected calls, settings or dependencies, inspect what
libraries already in use provide. Record the source in the row rather than relying on a separate research report.

Write `$ARTIFACTS_DIR/plan/draft.json` using `$ARTIFACTS_DIR/plan/draft.schema.json`. Write only structure decisions: placement, ownership, dependency direction and boundaries. Leave `increments`, `tests` and `kept_behaviors` empty for the detail phase. Keep the IDs of retained rows. Rows applying agreed decisions carry their `ref` and preserve the agreed choice. Include pending structure rows using their existing IDs.

Every new structure row has `nature`: `kind`, `reasons_for_change` (who changes it and why), `volatility` and `callers`. Each is either `unknown` or `{ "value": "...", "evidence": "path, request words or call relation" }`. Each row's `evidence` uses `source: machine` or `source: author`, with `text` naming where to verify the fact.

Write the skeleton in the checkout; leave committing to the workflow.

- Only structure: needed files and folders, full exported function, type and constant signatures,
  imports between them, and entry-point wiring that makes the parts reachable. Delete removed files.
- New bodies only fail loudly, using the language's closest construct for an unimplemented body.
  Write no new logic, tests or comments. In existing files change only signatures, exports, imports
  and new declarations needed for this skeleton.
- When restructuring existing code, move existing bodies unchanged so behavior and tests remain intact.
- Use the named structure criteria when placing items; existing layout alone is not placement evidence.
- Change nothing the requirements do not need. The workflow runs only configured `skeleton_check`
  names; an empty list means no skeleton check, not an assumed stack-specific command.

For each row use the draft schema's `id`, `what`, `chosen`, `candidates`, `principles`, `evidence`,
`decided_by`, `when`, `decide_when`, `quality` and `touches`. `touches` names actual paths; a folder
covers descendants. Use `when: later` with what must become known in `decide_when`, otherwise `now`
with an empty `decide_when`. Use the named authority document for attribution. An owner row's
`what` is its complete question; `chosen` is the proposed choice until a matching answer settles it.
Keep bound answers with their target; do not transfer an answer to a revised row.

Return the node's JSON object: `prototype` (empty, or the complete owner question requesting what a separate prototype must establish and why this plan cannot proceed without it; request the prototype on a separate request and branch, and write no skeleton), `structure_question` (the complete owner question about the skeleton), `provisional` and `uncertain` arrays. Write owner-facing texts in input `language`.
