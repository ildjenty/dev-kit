## Purpose

Repair the blocking findings for this commit within the agreed requirements. Other sessions made the findings and plan; verify their evidence yourself before changing code. Checks run after you, the workflow commits the repair, and fresh code judges examine the whole change again; a symptom-only fix makes the loop repeat.

Read `$ARTIFACTS_DIR/fix/input.json` and the fixed documents named by `writer_materials`:
`criteria`, `common`, `authority`, numbered `principles` and `every_quality` references (path before
`#`). Apply these standards to the repair.

Read `$ARTIFACTS_DIR/fix/findings.json`, `requirements.json` and `plan.json`; requirements and plan
may be null for review-only work. Finding IDs include `<judge-run>/<judge>/<finding-id>`; preserve
the full reference. On later rounds read `$ARTIFACTS_DIR/fix/state.json` for accepted/rejected
objections and `$ARTIFACTS_DIR/checks/round` and its corresponding JSONL/logs for check results.
Repair the supplied findings in the checkout. The machine commits; use configured check commands
if testing during writing.

Fix the cause shown by each finding's evidence and the check logs. Change only what the findings
and requirements need. Preserve tests and expectations; do not weaken, skip or delete tests, or loosen
a completion condition or plan choice just to clear a finding. Use the named standards when a real
contract discrepancy requires a decision instead of a repair.

Object only when observed evidence shows that the failure cannot happen in the current code,
the cited criterion does not apply, or the proposed repair would break a completion condition or
owner decision. Difficulty of repair is not an objection. To object to a finding, write `$ARTIFACTS_DIR/objections.json` as
`{ "objections": [{ "finding": "<complete ID>", "evidence": "..." }] }` for this pass. Name the
observed files, lines and command results. The independent dispute step decides; retained accepted
objections remove their findings from repair, and rejected objections must be repaired without
repeating the objection.

Return the node's JSON shape in input `language`:

- `fixed`: `{ "finding": "<complete finding ID>", "change": "..." }` for each repair.
- `broken_decisions`: `{ "row": "<plan row ID>", "observation": "..." }` when the plan premise fails;
  the step-back judge chooses the destination.
- `owner_decisions`: `{ "target": "finding|decision", "ref": "<ID>", "question": "..." }`
  for choices reserved by the named authority document. Leave dependent changes unwritten; the
  machine returns `back` to plan, where the owner is asked.
- `commit_message`: one line with a prefix from fixed `git.md`; the machine adds provenance trailers.
