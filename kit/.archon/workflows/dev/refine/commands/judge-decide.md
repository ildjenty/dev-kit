## Purpose

Judge the decision log, binding edits and Issue drafts independently. You did not write them; verify the owner's sources and the committed checkout yourself. The owner approves this version before Issues are created, so an invented choice, weakened rule or oversized work unit would misdirect delivery.

Read `$ARTIFACTS_DIR/refine/input.json`, `draft.json`, `approval.txt`, `check.json` in the same folder and
all documents named by `judge_materials`. Read the existing decision records in full, especially
those cited, depended on or superseded. Read `harness.md`, `review/harness.md` and the standards'
rules for changing standards relative to the criteria folder given in the input.
Read the actual binding files in the checkout. Use Read, Grep and Glob to verify evidence.
The machine owns schema, graph, reference, path scope and clean proposal commit checks.

Return exactly one criterion for each aspect:
- `coverage`: the request, confirmed memo and answers survive in the decisions and Issue drafts.
- `additions`: each choice has its actual source and follows authority; derived choices are not owner choices.
- `open_points`: unanswered questions become topic drafts, with the information needed to settle them.
  Group questions holding the same units at the same time only when they cannot be decided apart;
  require grouping by this rule rather than one Issue per question. Separate questions holding
  different units or independently decidable questions. Questions holding no unit go in separate,
  non-blocking topics; closely related ones may share a topic. Mixing questions that hold different
  units, or including a question a waiting unit does not need in its blocking topic, is blocking.
- `consistency`: decisions, bindings, supersession and Issue dependencies agree.
- `sufficiency`: the decisions give background, candidates, choice and reason.
- `wording`: the decision log and Issue drafts say precisely what the owner is being asked to approve.
  Check `pr_title` and `pr_summary` as text for this PR's reviewer in the language named by
  `language` in your input: the short title names this change; the summary explains what merging
  puts on the base and why, without approval instructions, versions or internal history.
  These fields remain in the full proposal details and are used unchanged for the PR.
  Check that the approval question contains only: every choice or rule added or derived beyond the
  confirmed memo and owner's words (including provisional decisions, derived agent rules and defaults),
  each keyed and summarised in one or two lines with a recommendation and reason; every risk the
  owner must accept, keyed and summarised in one or two lines; one line saying everything else follows
  the confirmed memo and owner's words with `devkit proposal show <branch>` for details; and how to
  answer with `approve=<version>` or `answers` naming the key to change. A missing memo-external item
  or risk is blocking. Repeating what the memo or owner already settled is a nonblocking finding
  asking to cut it. Report other unnecessary content as nonblocking findings asking to cut it:
  version history, provenance inventories, machine fields, JSON, full bodies, exhaustive binding/Issue
  lists and facts needing no decision (including cost estimates). The version belongs only in the answer line.
  When only these nonblocking cuts remain, keep the criterion `pass` and `needs_decision: false`.
- `bindings`: every binding edit is about how agents work: process, tools, conventions or verification.
  A product data shape, invariant or feature rule in a binding edit is a blocking finding, even if
  repeated in an Issue draft. Decisions about agent work that must bind agents use the edited mechanisms. Apply the
  standards' rules for changing standards and harness, including evidence and ongoing burden.
  A choice recorded only for people needs no binding edit. Delivery never consumes decision records.
- `issues`: each unit fits one PR and has observable completion conditions without requiring a
  decision record. Every required product property appears as a testable completion condition in
  the unit that builds it. For a shared property, check the first unit that builds it and repetition
  of the condition in later units that touch it. Missing conditions are blocking findings.
  Review the new `parent` title and body like every other Issue draft, against the
  owner's sources and the configured language. Parent/sub-issue and blocked-by structure expresses
  order; questions remain topics. The full proposal details include the new parent's title and body.
  A unit draft's work content is only its goal, observable completion conditions and product
  properties to preserve; an open question in a unit draft is blocking. Each topic body must name
  the units it holds or explicitly say it holds none. Missing declarations or a mismatch with the
  waiting units' `blocked_by` links, including any unit blocked by a topic holding none, are blocking.

Use the supplied common rules, criteria and authority for verdicts and findings. Write in the
language named by `language` in your input. Return every property required by the output schema.
Use empty strings or arrays and `false` for inapplicable values. Write no files.
