## Purpose

Turn backlog evidence into proposed work units, checks and standard corrections. Other sessions produced the findings and objections; verify their source judgments yourself. The retro judge and owner review this proposal before it is applied; ignoring resolved failures or weakening standards without evidence prevents learning from the work.

Read `$ARTIFACTS_DIR/retro/input.json`, `$ARTIFACTS_DIR/retro/draft.schema.json`,
`$ARTIFACTS_DIR/retro/check.json` and `$ARTIFACTS_DIR/retro/rounds/latest.json` when present.
The input contains all backlog entries, their original judgments and objections, the config,
concern map, standards index, previous proposal, recorded answers, and `language`.
Read the fixed documents named by `writer_materials`: criteria, common rules, authority and
numbered principle references (path before `#`), including `harness.md` and `review/harness.md`.

Write `$ARTIFACTS_DIR/retro/draft.json` matching the supplied draft schema:
- `summary`: the proposed correction.
- `units`: work for deferred or carried findings, with stable draft `key`, `title`, `goal`,
  `completion_conditions`, and source `entries`.
- `checks`: checks for mechanical findings, with the config check fields, `reason` and `entries`.
  Add precisely those checks to `.devkit/config.json`, retaining all existing settings and checks.
  An empty `paths` array selects no files; use applicable repository globs.
- `standards`: edits for accepted objections or recurring findings, naming `path`, `reason`, `evidence`,
  and source `entries`. Make these edits in the checkout, and trace every changed standard document.
- `dismissed`: source `entries` and a reason for each entry that needs no correction.

The input also lists existing units with `state`. A unit with `state: done` is finished work:
preserve its file unchanged and propose a new unit for follow-up work; never edit or delete the done unit.

Keep keys of retained units when revising a proposal and omit withdrawn ones. Use the exact entry IDs supplied in input. Cover every entry with a proposal or dismissal.
Write units only in the draft; the machine materializes them on approval. Edit only the config
and the configured standards folder. Do not commit. Revisions must address the previous round and answers.

Write `$ARTIFACTS_DIR/retro/approval.txt` in `language`: one approval question describing the entire
proposal, including dismissals, source entries, failure prevented, evidence and ongoing burden.
Include the exact `approve=<next_proposal_version>` and explain `answers` for requesting changes.
The machine commits the proposal JSON and restores the proposed config and standards until approval.

Return only `{"draft":"<ARTIFACTS_DIR>/retro/draft.json"}`.
