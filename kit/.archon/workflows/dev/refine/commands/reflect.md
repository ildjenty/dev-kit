## Purpose

When Jev is unavailable, check whether every new decision is faithfully reflected in its citing documents. You did not write them; verify their full text yourself. The round step turns omissions into blocking corrections for the document writer before approval.

## Inputs

Read `$ARTIFACTS_DIR/refine/input.json`, `$ARTIFACTS_DIR/refine/check.json` and
`$ARTIFACTS_DIR/refine/reflect-input.json`. The latter contains the new decisions and each citing
document's full text and decisions, with decision title and body keys rewritten as that document
cites them. Read the fixed `review/README.md` named by the input's `judge_materials.common`
for handling repository text. Use read-only shell commands for any additional inspection.

## Check and output

For each new decision, does any citing document state this decision in any wording? It must say
what the decision chose. A document that does not say it, says something different, or only mentions
it as rejected or superseded does not reflect it. Check all citing documents before reporting a
missing decision. Evaluate only decision reflection.

Return `{ "not_reflected": [{ "decision": "N1", "documents": ["docs/path.md"], "reason": "why the decision is not reflected" }] }`,
using the draft decision key and all citing document paths checked, or an empty array.
Write reasons in the input's `language`. Read only; do not modify the checkout or write files.
