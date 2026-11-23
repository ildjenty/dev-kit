## Purpose

Write the prose for a pull request whose machine facts remain visible. Other sessions wrote and judged the change; verify their evidence yourself. The report judge reads the assembled body, then the owner decides whether to merge from it without reading code. Unsupported claims or missing verification steps mislead that decision.

Read `$ARTIFACTS_DIR/pr/facts.json` and `writer_materials`. Read the named `criteria`, `common`, `authority` and numbered `principles` references (document path before `#`).

# Write the pull request

Read `$ARTIFACTS_DIR/pr/facts.json` first for `language`, `standards` and the fixed `criteria_folder`. Read `requirements.json`, `diff.patch`, `judgments.json` and `lists.json` in that same `pr` folder. The machine lists are assembled verbatim into the body; you supply only the prose the script cannot infer. Read the relevant code to substantiate claims.

Write in the language named by `language` in your input. Return:

- `title`: one line describing the resulting behavior.
- `opening`: what changes for users or operators and why.
- `how_to_see`: how the owner can verify the result without reading code. Cover conditions using `owner_check` or `screen_record`. Distinguish completed checks from instructions for later verification. Claim only evidence actually present in the materials.
- `after_merge`: necessary work after merge in execution order, including who provides any required values. Empty when no work remains.
- `headings`: localized headings named `what_changed`, `how_checked`, `what_remains`, `decisions`, `after_merge`, meaning what changed, how it was checked, what remains, decisions, work after merge.

Use `writing.md` and `verification.md` in the fixed criteria folder. When `report-findings.json` exists, read the previous body in `pr.md` and address its blocking or owner findings. Include no work history or invented deployment steps. Do not write a file or omit inconvenient machine facts.
