## Purpose

Reflect the settled topic in documents and cut work units only for settled areas. You did not necessarily write the topic's decisions; verify their full bodies yourself. The document judge and owner review this output, then requirements consumes approved units; stale prose or an early cut passes an incomplete decision downstream.

## Inputs

Reflect the settled topic in prose documents and cut work units for areas whose topics are settled.
Read `$ARTIFACTS_DIR/refine/input.json`, `$ARTIFACTS_DIR/refine/decided.json`,
`$ARTIFACTS_DIR/refine/draft.schema.json` and the input's `writer_materials` (named criteria,
common rules, authority and numbered principles). Read documents, units and affected items in full. For a continued unapproved proposal, use
`previous.content.documents` to recover its proposed prose before applying the recorded answers.
Read `$ARTIFACTS_DIR/refine/document/rounds/latest.json` when present for corrections.
Write owner-facing text in the input's `language`.

Read `relevant_decisions` in full. The `decisions` field is the index of every decision.
Read any other decision file by its indexed `path` when its title suggests it matters.
Any decision the draft supersedes, depends on or cites must be read in full.

## Writes

Update ordinary prose only under `folders.docs`, citing the settled decision IDs or `N1`, `N2`, ...
keys. Every new decision must be reflected in a document. Inspect affected documents and units,
including indirect dependencies: update documents or explain why they remain accurate in
`kept_docs`; record each affected unit's disposition and reason in `kept_units`.
When superseding a decision, retain every citing open unit in `units` with successor citations (use its existing unit ID as the key, or its previous `UN` key for an unapproved proposal), or withdraw it where already allowed; `kept_units` alone is insufficient.
A unit whose `state` is `done` has finished work: preserve its file unchanged, including when it is affected.
Explain its disposition in `kept_units` and cut a new unit for follow-up work. Never edit or delete it.
Do not edit decision, unit or topic record files directly, even when located under the docs folder.
Do not commit or edit other checkout paths.

Write `$ARTIFACTS_DIR/refine/draft.json`, preserving `decisions`, `rebased`, `topics` and
`open_points` exactly from `decided.json`'s draft. Add `units` only for settled areas. The machine treats
a unit's cited decisions and their transitive dependencies as its area; an open topic depending on
any of those decisions, including through its own dependencies, prevents that cut. A topic without dependencies keeps the request open.
The current topic is settled prospectively for this check; its status becomes settled on approval.
Each unit is one reviewable PR with a goal, observable completion conditions and supporting decisions. Use `UN1`, `UN2`, ... keys,
retaining previous keys for retained units and omitting withdrawn ones.

Write `$ARTIFACTS_DIR/refine/approval.txt`: list each decision, next topic and unit by key and title,
separating owner, principle and provisional decisions under authority.md. Explain how to restart
with `approve=<next_proposal_version>` from the input, or `answers` requesting changes.

Return `{ "draft": "<absolute path to draft.json>" }`.
