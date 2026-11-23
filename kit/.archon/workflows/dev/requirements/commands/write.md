## Purpose

Turn the request into a goal, completion conditions, constraints and open points. Earlier sessions supplied units, decisions and request supplements; verify their sources yourself. The requirements judge, plan writers and code judges consume this output: what is lost here is never built, and what is added here is built without being requested.

Read `$ARTIFACTS_DIR/requirements/input.json`: `requests` keeps `owner_words` and session
`supplement` apart; `unit` and `agreed_decisions` supply the selected work unit and constraints;
`previous` holds the prior version; `answers` holds addressed owner answers. Read the fixed documents
named by `writer_materials` (`criteria`, `common`, `authority`, numbered `principles` and
`every_quality`, using the path before `#`). Apply `review/purpose.md` and these standards.
Read the repository documents needed for the request and the supplied Issue/PR text in supplement.
Address `returned` premises and observations first.
For superseded decisions in `returned`, change only items that rest on the old decision; keep everything else as it is. When a successor changes nothing for an item, only move its citation to the successor.
Read all requests in order: an empty
`owner_words` carries a session fact, not a new owner request. Keep what still holds from `previous`
and integrate later requests without losing retained completion-condition or open-point IDs.
On later rounds read `$ARTIFACTS_DIR/requirements/rounds/latest.json` for schema errors and judgments.

Correct listed schema errors, blocking findings and failed or unavailable assessments you can
address, and preserve the rest. A means explicitly named by the owner is a constraint; a means
named only by the session is a candidate. Never fill ambiguity with a guess: when its answer changes
what gets built and the code or documents do not settle it, record an open point. For an urgent
request, retain only what stops the damage; the rest belongs to another request.

Return the node's JSON shape in input `language`:

- `goal`: one or two sentences stating the problem, intended result and whom it serves, in the request's terms.
- `completion_conditions`: `{ "id": "C1", "text": "..." }` entries; retain existing IDs.
- `constraints`: strings with their sources, including D-IDs for agreed decisions.
- `open_points`: `{ "id": "O1", "text": "..." }` entries; retain existing IDs. The text is the
  complete owner question, using the named authority and writing standards.

Preserve repository paths verbatim in backticks so plan can locate them. Return structured output;
the machine writes the requirements record. These fields describe the requested product or code, not this workflow. Write no files.
