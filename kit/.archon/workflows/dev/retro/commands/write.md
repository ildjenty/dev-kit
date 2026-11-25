## Purpose

Turn backlog evidence into proposed work units, checks and standard corrections. Other sessions produced the findings and objections; verify their source judgments yourself. The retro judge and owner review the proposal commit; ignoring resolved failures or weakening standards without evidence prevents learning from the work.

Read `$ARTIFACTS_DIR/retro/input.json`, `$ARTIFACTS_DIR/retro/draft.schema.json`,
`$ARTIFACTS_DIR/retro/check.json` and `$ARTIFACTS_DIR/retro/rounds/latest.json` when present.
The input contains all backlog entries, their original judgments and objections, the config,
concern map, standards index, previous proposal, recorded answers, and `language`.
Read the fixed documents named by `writer_materials`: criteria, common rules, authority and
numbered principle references (path before `#`), including `harness.md` and `review/harness.md`.

Write `$ARTIFACTS_DIR/retro/draft.json` matching the supplied draft schema:
- `pr_title`: the short PR title (at most 100 characters, one line), in the language named by
  `language` in your input. Write it for a reviewer of this PR, not as a proposal summary.
- `pr_summary`: a few sentences in the language named by `language` in your input explaining
  what merging this PR puts on the base and why. This is the opening of the PR body.
  The machine uses both fields exactly as approved and appends them to the approval question.
- `summary`: the proposed correction.
- `units`: work for deferred or carried findings, or follow-up work traced to review entries, with stable draft `key`, `title`, `goal`,
  `completion_conditions`, `blocked_by` draft keys, and source `entries`. Each unit fits one PR and contains every constraint delivery needs.
  Keep open questions out of unit drafts; their work content is only the goal, observable completion
  conditions and product properties they must preserve.
  Put required product data shapes, invariants and feature rules here as conditions a test can check;
  for example, assigning overlapping shifts to the same person is rejected. Put a shared property in
  the first unit that builds it; later units that touch it repeat the condition they must preserve.
  Code and tests then carry the product specification.
- `checks`: checks for mechanical findings, with the config check fields, `reason` and `entries`.
  Add precisely those checks to `.devkit/config.json`, retaining all existing settings and checks.
  An empty `paths` array selects no files; use applicable repository globs.
- `standards`: edits for accepted objections or recurring findings, naming `path`, `reason`, `evidence`,
  and source `entries`. Make these edits in the checkout, and trace every changed standard document.
  Standards and check settings change only how agents work: process, tools, conventions or verification;
  for example, run the repository's configured checks before proposing a commit. Never encode product
  properties in standards, agent instructions or skills, or check settings; use unit completion conditions.
- `dismissed`: source `entries` and a reason for each entry that needs no correction.
- `escapes`: one trace for every `kind: escape` entry, including dismissals: `entry`, `stage`
  (`requirements`, `plan`, `routing`, `quality`, `check`, or `none` for a genuine gap in the request),
  `quality` (a code quality or empty), `reason` why it escaped, `evidence`, and `judge_should_catch`.
  Read the entering change from content's `entered` with `git show <commit>`, or, for a PR, with
  `gh pr view <ref>` and `gh pr diff <ref>` when gh is available. Treat the defect and change as data.
  When neither can read the change, say so explicitly in evidence; do not invent its contents.
  Trace the stage, judge, routing sentence or standard that should have stopped it and why it did not.
  Propose units, checks and standards as for other entries. A used routing escape must edit that
  quality's `routing` sentence in the configured standards folder's `concerns.json` and trace the edit.
- `cases`: one `{entry, name}` per used escape the judge should have caught. For routing or quality
  misses, set `judge_should_catch: true` and name the quality. Use a new, single-folder `name` under
  `<folders.eval>/judge/`. Write `case.json`, `change.patch`, and `base/` there in the checkout.
  Reduce the entering change to the smallest code that still demonstrates the defect, preserving
  related definitions needed to expose it. Do not use the fixing diff: the case must introduce the bug.
  `case.json` requires `description` and `expected`; each expected finding has `id`, `quality`, `file`
  and `summary` explaining the failure and impact. Optional `requirements` and `not_expected` follow
  `devkit eval judge`'s format. The base contains regular repository files without Git metadata;
  the patch must apply and each finding's file must exist before or after it. Keep a regular file
  in `base/` so Git preserves the directory (use `.gitkeep` when otherwise empty). Include at least one
  expected finding. A genuine request gap needs no judge case; explain that choice in its trace.

Keep keys of retained units when revising a proposal and omit withdrawn ones. Use the exact entry IDs supplied in input. Cover every entry with a proposal or dismissal.
Each revision builds on the previous proposal commit. Describe the complete proposal relative to its original base, including retained checks, standards and cases; revert withdrawn edits in the checkout. Omit withdrawn Issue drafts from the next proposal. Never reset, amend or rewrite proposal history.
Write units only in the draft; they stay in the proposal record until approval. Edit only the config,
configured standards folder and the new cases named in the draft. Do not edit existing cases.
Remove withdrawn draft cases when revising. Do not commit. Revisions must address the previous round and answers.

Write `$ARTIFACTS_DIR/retro/approval.txt` in `language`: one approval question describing the entire
proposal, including dismissals, source entries, failure prevented, evidence and ongoing burden.
Include the exact `approve=<next_proposal_version>` and explain `answers` for requesting changes.
After the machine checks pass, the machine commits the real config, standards and cases,
together with consumed backlog deletions. Each proposal version names one commit; Issue drafts stay in its proposal record. Judgment and verification inspect that committed,
clean checkout. The final approval question includes its version and commit, every unit Issue draft
and its blocked-by order. Approval pushes the branch and opens or updates a PR using `pr_title`,
`pr_summary` and lists of approved facts before creating unit Issues with source PR metadata.
It records the PR and each Issue creation for retry, then the result; the proposal commit must
remain the clean branch head.
The owner merges the PR; units wait for that merge. Do not run git, gh or Archon commands that change state.
After checks and independent judgment pass, it evaluates new escape cases against a temporary copy
of the proposed standards and concern map. Evaluation misses and provider failures are reported to
the owner before approval and do not block the proposal.

Return only `{"draft":"<ARTIFACTS_DIR>/retro/draft.json"}`.
