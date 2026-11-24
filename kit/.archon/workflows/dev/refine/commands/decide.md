## Purpose

Settle exactly one topic into decision proposals and the next questions it opens. Earlier sessions supplied requests and records; verify their complete sources yourself. The decide judge and document writer consume this output, and approved records constrain downstream requirements; losing an owner request or guessing an answer changes what gets built.

## Inputs

Settle exactly the topic in `$ARTIFACTS_DIR/refine/input.json`. Read the whole input, including
`topic` (question and required answer), requests, memo, answers,
previous proposal, topics, affected items and `writer_materials`. Read the named criteria,
common rules, authority and numbered principle references. These fixed standards govern this step.
Read request and answer texts in full, keeping owner words and confirmed memo apart from session
supplement. Read the question beside each answer. Preserve the owner's terminology. The previous
proposal is a proposal to revise, not permission to attribute new choices to the owner.
Owner-facing text uses the input's `language`. Read `$ARTIFACTS_DIR/refine/decide/rounds/latest.json`
when present and address its machine errors, findings and unavailable assessments.

Read `relevant_decisions` in full. The `decisions` field is the index of every decision.
Read any other decision file by its indexed `path` when its title suggests it matters.
Any decision the draft supersedes, depends on or cites must be read in full.

## Writes

Write only `$ARTIFACTS_DIR/refine/draft.json`, following
`$ARTIFACTS_DIR/refine/draft.schema.json`. Do not change the checkout or commit.
Write decisions for this topic and the next topics it opens, not decisions answering those next topics.
Use `N1`, `N2`, ... for new decision keys and `TN1`, `TN2`, ... for next-topic keys. Retain keys
when revising a previous proposal; omit withdrawn keys. Only the machine writes record files.

Each decision's `body` gives background, candidates including the simplest alternative, selection,
reason and uncertainty. Its `source` traces the decision to the input; use authority.md for attribution.
Agreed decisions are constraints. Replace one only through `supersedes`. Use `rebased` for a dependent
decision only when changing its dependency to the successor preserves its substantive meaning.
Each next topic has `title`, `body` containing its question and what a settled answer must contain,
and `depends_on` listing supporting decision IDs or draft decision keys.

Keep IDs of retained open points. For an unresolved fact about the owner's situation, ask for the
fact without a recommendation. For an unresolved choice, supply candidates, a recommendation and
its reason under the named authority document. Do not reopen a point settled by an answer or source.

Write unresolved points needed to settle THIS topic in `open_points`. Keep `units`, `kept_docs`
and `kept_units` empty; documents and the final work cut belong to the document phase.


Return `{ "draft": "<absolute path to draft.json>" }`.
