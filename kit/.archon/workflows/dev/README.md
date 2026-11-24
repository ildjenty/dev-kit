# Development workflows

The owner decides what matters, changes stay local, and observed failures improve the process. Archon runs nine workflows; `devkit` performs mechanical checks and records transitions. The installed `devkit-discuss` and `devkit-runs` session skills handle conversation, starting runs and relaying results. Workflows never start each other.

## Installing and updating

`devkit init [--yes | --no] [--dry-run] [dir]` compares everything before writing. For the workflow folder and each installed skill folder, it lists differing and extra files and asks `replace N files and delete M files? [y/N]`. No preserves those files and allows missing files to be added. Stale files are deleted only after Yes, only within these kit-owned folders and a renamed skill's previous folder. Each differing configuration file has its own question. Existing standards at the configured path are replaced as a whole only after Yes; No adds nothing, including missing files. Identical files are silent; absent files need no confirmation.

Use repeatable `--skill` flags to choose distinct installed names made of lowercase letters, digits and hyphens. The `skills` mapping in `.devkit/config.json` remembers them. Installed skill folders, front matter, cross-references and watch-script paths use those names. An explicit flag updates only `skills` even when config replacement is declined. The standards destination is retained when config is replaced. Use `devkit config` to read effective names and paths.

Destination and ancestor symbolic links are named and asked about. Yes replaces the link itself without touching its target; No skips affected destinations. A declined group prevents replacing a shared ancestor link that would remove its files. `--yes` and `--no` answer all questions; stdin without a terminal defaults to No with a notice. `--force` is unsupported. `--dry-run` prints comparisons and questions and changes nothing. Init lists written, deleted and retained paths and exits successfully when changes are declined. Malformed JSON config can be kept or replaced using default destinations for comparison; applying only `skills` requires a JSON object. Invalid destination settings are errors. Review and commit the result before running workflows.

## Sequence and handoffs

Init writes `.devkit-installed.json` in this folder with `devkit_version` and `kit_digest` when the workflows match its kit after a fresh install, Yes, or no differences. No keeps the previous stamp, including when missing files are added. The stamp belongs to the workflow comparison and is never an extra file; hand edits do not update it. `devkit version` prints the binary version and the embedded kit's SHA-256 digest, stable for the same paths and contents across builds.

Every start checks this workflow's YAML and command files in `$PACK_DIR` for unregistered commands before saving records or answers. A missing command returns `not_started` with the command, the kit/binary mismatch explanation, and remedies: run `devkit init` in the repository with the matching binary or install the binary that installed these workflows (the stamp names its version). Relay this to the owner and do not retry. Digest differences alone do not prevent a start.

Use `devkit-discuss` to produce an owner-confirmed memo, then `refine` on a `refine/<name>` branch. Refine settles one topic through decide and document phases, each with at most three rounds. The document loop runs machine check → Jev reflection → configured provider CLI reflection only if Jev is unavailable → configured provider wording judge → round. Every new decision must be cited by at least one document before reflection runs. Pass `topic=T-<id>` for an open topic listed by `devkit decisions topics`; an empty topic starts from the request itself. After the owner approves the recorded version using `approve=<version>`, publish and merge the refine PR through the repository's PR process. Refine commits its proposal and agreement but does not publish a PR.

Only after that merge, start each eligible unit on its own branch: `requirements` with `unit=U-<id>` → `plan` → `implement` → `judge` ⇄ `fix` → `pr`. Use `devkit decisions units` in the base checkout and start open units marked ready; `--json` provides `id`, `title`, `depends_on`, `state` and `ready`. Requirements ends `not_started` for a done unit or one waiting for prerequisites. Direct settled requests use requirements' `owner_words`; session observations go in `supplement`. Plan is required for delivery. Review-only `judge` also works without requirements or a plan. Use `sync` to rebase; run judge when its result says `needs_judge`, then return to PR.

After backlog reaches the base branch, run `retro` on a `retro/<name>` branch created from that base. It proposes units, check additions, standard edits or reasoned dismissals; its approval applies the changes and deletes consumed backlog files in one commit. Publish and merge the correction PR before starting its units.

Record defects found after merge, including small direct fixes, with
`devkit backlog escape --defect-file <file> --entered <commit-or-PR> [--fixed <commit-or-PR>]`.
The file preserves the owner's description without command-line text. References accept locally
resolvable commits (stored as full hashes), commit hashes, GitHub PR URLs or positive PR numbers.
Commit the printed `.devkit/backlog/escapes/<random-hex>.json` with the fix or through the PR process.
Retro reads it as `kind: escape`, with ID `<path>#escape/0`, and requires a used/dismissed decision
and a trace of the entering change and missed stage. Routing misses correct the quality's `routing`
sentence in the standards folder's `concerns.json`. Each used escape the judge should have caught
adds a minimal `base/`, `change.patch` and `case.json` under the configured `<eval>/judge/<name>/`.
The retro check reuses judge-evaluation validation. After checks and judgment pass, `verify` runs
only the new cases against a temporary copy of the proposed standards and concern map. The AI
reports found/missed expectations, routing misses and provider failures in the approval question's
configured language. Misses and provider errors do not block the proposal. This incurs provider
calls only for escape cases. Cases are restored until approval, then applied with the corrections
and deletion of the consumed records in one commit.

A `question` result ends the run. Relay every question and ID and restart its named `next_workflows` workflow with verbatim owner answers. Refine and retro approval use the exact recorded version. Requirements and plan ask unresolved request/design questions; implementation, judge and fix return `back` to plan for owner decisions. PR asks only for uncovered `ask` changes. Follow `back.to`; `stop` describes unresolved work, and `not_started` means prerequisites rejected the start. `carry` is a plan result carrying detail findings forward. Counted restarts and fix runs obey configured limits; supply owner words as `continue` only when the restart limit is reached.

When every failing check is `preexisting`, implement/fix stop immediately without stuck, step-back or pass consumption. Judge includes base failures in its judgment and stops when any check is preexisting instead of recommending fix. The stop records `base_failed: true`; loops end `base_failed`. Relay the failing checks and their log paths and tell the owner to fix the base branch first, then start the same workflow again. `devkit record limit` classifies that restart as `retry_after_base`, not counted against `limits.restarts`, even at the limit. `devkit status` explains this. Retrying before the base is fixed is pointless. Regressed and unavailable comparisons retain the existing implement/fix behavior. Implement and fix call `devkit checks --base-on-repeat`: a failing check is compared with the base only when it also failed in the previous pass (others record `base: not_compared`); judge and sync compare every failing check.

## After the pull request

Read `gh pr view <branch> --comments` and `gh pr checks <branch>`, then read the actual inline comments
or failing job logs. An in-scope repair or CI failure goes to `fix` on the same branch as `review`,
verbatim with provenance (`Review comment by <login> on path:line: ...` or
`CI job <name> failed: <log excerpt>`). Review starts need passed requirements and plan and the latest
judge's change matching HEAD by PR identity, including after the backlog/done-marker commit.
The run stores the review text and starting commit; its fix findings have source `review`.
Items outside requirements or plan, and disputed review items, appear in `outside_requirements`
with `item` and `reason` and return `back` to requirements with those items as premise and the full
review as observation. Review objections bypass the dispute judge. Checks and fix/restart limits
still apply; this return makes no fix commit. A pass needs a new judge run before PR, even if the
change identity stayed the same; PR then updates the existing pull request.

A comment that changes or adds to the request goes directly to requirements on the same branch as
`owner_words`, then follows the usual flow. A recorded unit request can resume after its successful
PR when its valid done marker names that branch. Never answer or resolve comments on the reviewer's
behalf. For a CI failure, suggest adding the job through `devkit-optimize` as a configured check.

After a passing PR, status optionally runs `gh pr view <branch> --json state,url,reviewDecision,statusCheckRollup`
with a 15-second timeout. It shows state, URL, review decision and failing checks in text and JSON;
lookup failures are shown and never fatal. `next` routes open failures to fix with `review`, requested
changes to fix with `review` or requirements when the request changes, merged PRs to
`archon complete <branch>` and closed PRs to closed without merging.
`archon complete <branch>` removes the worktree and local branch and keeps the remote; periodically
run `archon isolation cleanup --merged`.

## Concerns, decisions and structure

The code quality set is `internal/concerns`; the repository-owned `<standards>/concerns.json` assigns principles and plan/code criteria to structure, data, contract, security, ui, cost, test, delivery, scope and readability. Every numbered standard principle has exactly one assignment: one quality, `every_quality`, or a named writer/non-quality judge in `others`. Every named document exists and every review document except `review/README.md` is mapped. `devkit concerns check` validates the map, including during plan, judge, refine and retro prepare on fixed criteria.

Readability owns names, comments, conventions and shared rule definitions under `engineering.md#8`. For each added function, conversion, check or rule, it searches the whole judged tree named in `judge/target.json` for existing behaviour that must change together. Re-implementation blocks only when the rule or behaviour must change together; similar text with independent reasons for change or contracts stays independent. Placement and dependencies remain structure's responsibility. Structure routing includes function responsibilities and call paths as well as module placement; data routing includes valid states and domain rules as well as stored data.

Materials come from that map. `review/README.md` owns common judgment rules; `authority.md` owns owner decision boundaries. Commands specify inputs, writes and output contracts and refer to standards for rules. Quality criteria separate Decisions from Code, Blocking and Not reported. A choice passed in plan is checked for conformance in code rather than chosen again.

Repository decisions live in `<decisions>/D-<id>.md`, units in `<units>/U-<id>.md` and topics in `<topics>/T-<id>.md`. IDs derive from branch and draft key, starting with six lowercase hexadecimal characters and extending on collision. Re-proposals keep retained keys' IDs and remove withdrawn unapproved files. Dependency traversal reaches affected decisions and citing documents/units. Topics are pickable only when open and all decision dependencies are agreed. Requirements prepare loads the selected unit and agreed records as constraints. Plan rows apply them with `ref`; passed new owner/principle rows become agreed records in the skeleton commit. Provisional choices remain in the plan and PR body.

Structure comes first in plan. Nature evidence explains placement; its fresh structure judge must pass before detail adds other rows, release units and tests. Each phase has its own three-round loop. Detail can alter signatures but changing file membership or structure rows stops the run. Prepare writes the embedded plan draft schema and its reference dependency to artifacts. Writers cite facts in row evidence.

Judge computes `outside_plan`, folder spread and siblings on ancestor paths. Structural defects on the change's path block, including existing defects; those elsewhere are deferred. The bash `devkit judge route` node selects qualities from each changed file's diff with Jev (`jev-latest`). It asks one question per routable quality from the fixed concern map's `routing` description and selects the quality at or above its `routing_threshold`; a source/test question selects test and readability at 0.3. It uses at most four concurrent requests, 8 KiB diff chunks split at line boundaries and a 24 KiB encoded request bound. Each request has a 10-second timeout and one retry after 100 ms on timeout, 429 or 5xx. All probabilities, including those below the threshold and successful chunks from an unavailable result, are recorded in `judge/route-jev.json`. If any request fails or an answer is malformed, the entire selection falls back to the configured provider CLI with read-only tools/sandbox and schema validation. If both are unavailable, route selects every routable quality for every changed file with both failures in its reason. Its `source` is `jev`, `claude`, `codex` or `all`; launch reads this single result. The machine unions route, configured globs and plan rows; scope always runs, and source/test changes select test and readability. AI judges use the configured provider and fresh contexts on fixed criteria. Claude judges have read-only tool restrictions; Codex runs with full access. Both keep `mutates_checkout: false`: Archon fails a node after a detected checkout write. Codex does not prevent that write.

`devkit judge launch` writes `judge/launched.json` from the same decision as its `launch_<quality>` flags; `devkit eval judge` uses the same launch processing. Code quality judges read it and the fixed `concerns.json` ownership. They report failures of their own principles, or another quality's failure only when its owner was not launched. They receive no `every_quality` principles; these remain in requirements, plan, implement and fix materials. The machine checks `decision_paths` changes absent from plan rows, and scope checks authorization under `working.md#6`. A missing test or changed caller contract remains the respective quality's failure, even when another judge reports the defect itself.

Prepare keeps a single full judged tree at `$STATE_DIR/trees/<encoded branch>/`, outside records and run artifacts. `judge/target.json` and quality materials contain its absolute `tree` path. Judges read that tree with Read, Grep and Glob while concurrent checks may alter the checkout. Claude nodes retain `allowed_tools: [Read, Grep, Glob]` without path restrictions; Codex nodes have full read access. First use exports the whole commit; later runs apply only the Git diff, preserving bytes, executable bits and symlinks and deleting empty folders. Markers at `$STATE_DIR/trees/.commits/<encoded branch>` avoid reserving any repository filename. The marker is removed before updating and written last. Missing/interrupted markers, unavailable old commits or update failures cause a full re-export. Prepare also prunes trees for deleted local branches after `archon complete` or branch deletion, except the branch currently being judged.

`devkit refine prepare` selects relevant decision bodies once for all decide/document writer and judge rounds. `refine/input.json` contains `decisions`, an index of every decision (`id`, `title`, `status`, `depends_on`, `supersedes`, `path`), and `relevant_decisions`, the relevant full records. The required set includes the topic's transitive decision dependencies and citations, pending and previous proposal decisions, and their supersession chains in both directions, followed to closure. Decisions that open units cite alongside them are added one hop only; documents' co-citations are not followed, because an overview document would pull in everything. Other `agreed` decisions are selected together by one read-only call of the configured provider's CLI. `refine/relevance-input.json` supplies the topic title/body, requests' owner words and supplement, memo, and each candidate's ID, title, body and path; `always_ids` lists required decisions that need not be returned. Jev is not used for relevance because live API measurements did not separate relevant from unrelated decisions; the CLI selected more precisely without measured misses. This broad filter favors inclusion when uncertain, excludes unrelated features of the same system, and considers indirect constraints such as legal and calendar rules. Superseded and withdrawn decisions only enter through the required rules. `refine/relevance.json` records each decision's inclusion and source; if the CLI fails or returns an invalid selection, all agreed decisions are included and the failure reason is recorded. Writers and judges read the relevant set in full, inspect other indexed paths when titles suggest relevance, and read every superseded, depended-on or cited decision in full. Machine checks, materialization, reflection and unit-area checks retain their full checkout data.

The shared `internal/jev` client also serves `devkit refine reflect`, after a valid document check. Each new decision supplies a `noul` question with its title and body, rewriting draft keys as its citing document uses them. Document text is split at headings into chunks up to 8 KiB (oversized sections are split without breaking UTF-8); questions for several decisions citing the same chunk share one request. Encoded requests stay within 24 KiB, using the same concurrency, timeout and retry rules as routing. Any citing chunk at probability >= 0.5 reflects the decision. All probabilities are recorded in `refine/reflect-jev.json`; the command returns the source (`jev`, `claude` or `codex`), model when available and missing decisions with the highest probability and checked document paths. Any failed or malformed answer discards the partial result and runs the configured provider CLI with read-only tools/sandbox and schema validation. If both are unavailable, reflection exits with code 1 naming both failures. The wording judge and round read the single reflection result. Round turns each missing decision into a blocking `reflection` finding for the next writer round. `refine/reflect-input.json` holds the full citing documents and decision text for the fallback. Reflection criteria live in the Jev questions and `refine/commands/reflect.md`; the wording judge no longer judges reflection.

## Commands and artifacts

`devkit help` is the command inventory; `cmd/devkit` registers the mechanical packages. YAML passes Archon's `STATE_DIR`, `WORKFLOW_ID`, `BASE_BRANCH`, `ARTIFACTS_DIR`, declared `INPUTS_*` and producer outputs through environment variables. Commands return JSON on stdout, errors on stderr with a nonzero status; `record dir`, help and version return text.

- Refine: `devkit refine start`, `prepare`, `check`, `reflect`, `round`, `decided`, `finish`, `approve`, `not-started`. Artifacts under `refine/` include input, schemas, draft, decided snapshot, phase rounds and approval text. The document phase freezes decide output; scope checks inspect both index and worktree. Approval checks the started run and recorded answer before any repository update.
- Requirements: `devkit record start requirements`, then `devkit requirements prepare`, `round`, `finish`, `not-started`. Input and round artifacts are under `requirements/`; the writer returns requirements as structured output. Failed or unavailable criteria cannot pass.
- Plan: `devkit record start plan`, then `devkit plan prepare`, `facts`, `check`, `settle`, `advance`, `finish`, `not-started`. `plan/input.json`, `draft.schema.json`, `facts-before.json`, `draft.json`, `findings.json` and `structure.json` carry the two phases. Answers bind to requirements version and row content. Allowed provisional implementation updates record `derived_from` the accepted plan.
- Implement: `devkit record start implement`, then `devkit implement prepare`, `next`, `tests-written`, `end`, `state`, `finish`, and `devkit checks`. Separate sessions write tests and code per release unit. Repeated check failure invokes step-back. The workflow commits completed units; new owner choices go back to plan.
- Judge: `devkit record start judge`, then `devkit judge prepare`, `changes`, `launch`, `completion`, `collect`, `finish`, and `devkit checks`. `judge/` contains target, committed source snapshots, diff, decisions, locality facts, answers and per-quality materials. Without a plan, decision changes become `without_plan` materials.
- Fix: `devkit record start fix`, then `devkit fix prepare`, `proceed`, `objections`, `dispute-end`, `round-end`, `state`, `finish`, and `devkit checks`. An independent judge settles evidence-backed objections. Finding references include the original judge run, judge and local ID; acceptance cannot suppress another run's finding.
- PR: `devkit record start pr`, then `devkit pr answers`, `facts`, `assemble`, `judgment`, `state`, `open`, `finish`, and `output`. `pr/` holds facts, machine lists, body and title, report findings and publication result. Publication pushes to `origin` and uses `gh`; it does not merge.
- Sync: `devkit sync start`, `continue`, `state`, `finish`, `output`, and `devkit checks`. Before rebasing, sync writes mapped materials from the request's fixed standards into `sync/input.json`; conflict resolution receives them through `sync/conflict.json`. Failed resolution aborts the rebase and restores the original commit; successful conflict resolution sets `needs_judge`. Sync does not push.
- Retro: `devkit retro start`, `prepare`, `check`, `round`, `verify`, `finish`, `approve`, `empty`, `not-started`. `retro/` contains backlog input, schemas, draft, check, rounds, evaluation results and approval text. Proposed config/standard/case edits are restored until approval; an empty backlog passes without approval.

Shared commands include `devkit config`, `devkit criteria`, `devkit concerns check`, `devkit decisions list`, `topics`, `check`, `affected`, `dependents`, `next-id`, and `devkit record start`, `write`, `latest`, `dir`, `limit`, `result`. Consult help for arguments.

## Records and judged identity

Runtime records stay outside Git at `$STATE_DIR/records/<encoded-branch>/`; `feature/filter` encodes as `feature%2Ffilter`. Resolve the path with `devkit record dir` in the run environment. AI nodes receive selected artifacts rather than this directory. `devkit status [--json] [branch]` reads them from a session: it finds the state folder through `archon project list --json` (or `STATE_DIR`), and prints each branch's unit ID from its request record, last outcome, unanswered questions, stop or back details, and the next workflow to start.

`request/` preserves owner words, supplement and the initial criteria commit. `proposals/`, `requirements/` and `plans/` hold immutable versions; `judgments/` holds targets, findings and evidence. `runs/` records successful starts and restart accounting; `results/` records outcomes and input versions. `questions/` and `answers/` use question IDs, `allowed/` holds PR permissions, and `continuations/` holds owner authorization past restart limits. Every write validates against embedded schemas in `internal/record/schemas/`. Failed starts save no answers; starting an earlier stage invalidates later results.

Passing judgments cover HEAD or an identical merge-base change: file names and every added/removed line including whitespace match, hunk positions are ignored. The current branch's schema-valid backlog file and the valid done marker for its request unit naming that branch are excluded from identity and decision detection. Every other units-folder file remains part of the change. Code judgments store `change_id`.

## Backlog contract

Before pushing, PR commits `.devkit/backlog/<encoded-branch>.json` with a `chore:` subject. When the request record names a unit, the same commit includes `<units>/<unit-id>.done.json` with `unit` and `branch`, validated against embedded `unit-done.json`. Reruns keep the marker unchanged; branches without a unit write none. The unit becomes done on the base branch when its PR merges. `devkit decisions check` reports invalid markers or missing units. Refine and retro preserve done unit files and cut new follow-up units instead. The backlog's four required arrays are `deferred`, `carried`, `mechanical`, `accepted_objections`. Each entry has `finding` (the original recorded object) and `source` (`run_id`, one-based `judgment` record number, `judge`, `kind`, full `target`). Accepted objections are nonblocking dispute findings with the original finding reference. An entry may occur in multiple arrays. Retro input IDs are `<backlog-path>#<category>/<zero-based-index>` and preserve this content and provenance. The optional `review` array stores each review fix input as a finding with its full text, with source `kind: review`, fix `run_id` and `target.commit`; it refers to a run rather than a judgment. Retro reads these entries too. Consumed files are removed only with approved corrections.

## Configuration and verification

Repository-specific behavior belongs in `.devkit/config.json` and standards. Settings are `language`, `skills`, `standards`, `docs`, `decisions`, `units`, `topics`, `checks`, `skeleton_check`, `qualities`, `decision_paths` and `limits`. Use `devkit config` for effective values. Defaults install no checks. Check commands decide mechanical outcomes; exit 75 retries once as an environment error, 77 skips, 0 passes, and other exits fail. Both merge-base and current configurations supply checks so removal cannot bypass verification. Only failing checks are compared with the merge-base commit in a temporary detached worktree; passing checks and environment errors are not rerun there. The current repository's `.archon/config.yaml` supplies `worktree.postCreate`, run in order with `postCreateTimeout` milliseconds per command (default 300000). Preparation failure makes comparison unavailable, and the worktree is removed. Each failing result has `base: failed|passed|unavailable`, with `base_log` when run; gate output adds `preexisting` and `regressed`. `--no-base` skips comparison and is used by skeleton checks. Config itself is a default `ask` decision path.

For changes to this pack, edit the relevant YAML, prompts and Go decision points; cover changed outcome paths with stub fixtures and behavior with Go tests. Run from the source checkout:

```sh
mise exec -- go build ./...
mise exec -- go vet ./...
mise exec -- go test ./...
scripts/test-kit.sh
```

The script initializes a temporary repository and isolated Archon home, runs `archon validate workflows`, `devkit concerns check`, and `archon workflow test` on the fresh installation. It then copies `test/fixtures/` from source and reruns fixtures with real bash and stubbed AI through a temporary wrapper. `cmd/devkit-fixture` supplies prerequisite records and author file writes and verifies persisted results before scratch cleanup. Test helpers and executed fixtures are absent from `devkit` and the installed pack. PR publication is stubbed; sync executes a real local conflict. `scripts/test-kit.sh [workflow ...]` narrows the executed-fixture selection. Provider plugins default to `$HOME/.archon/plugins` with `KIT_ARCHON_PLUGINS` as override.

Requirements include Git with a commit, Archon and its configured provider (`archon provider add claude` or `archon provider add codex`), and devkit on PATH. `devkit init --provider claude|codex` renders workflow AI nodes; sources remain Claude. Codex rendering removes `allowed_tools`, `denied_tools` and `settingSources`; changing provider later prompts for workflow-folder differences. Every AI schema requires all properties, including nested objects, and closes objects with `additionalProperties: false`. Empty strings, empty arrays and false express inapplicable values.

`devkit init --sessions claude,codex` chooses session skill locations independently: `.claude/skills/` for Claude and `.agents/skills/` for Codex. Absent sessions defaults to the provider alone. Explicit provider/session flags are saved even when replacing config is declined. Each location gets its own paths, rename comparison and ownership protection for old folders. Names can be changed with `--skill`.

The configured CLI selects refine relevance and is the Jev fallback for judge routing and refine reflection. Codex uses `codex exec --sandbox read-only --cd <checkout> --output-schema <temporary schema> -o <temporary answer> --skip-git-repo-check -`; routing uses `<tree>` as its working directory, which has no Git metadata. Claude uses `claude -p --json-schema <schema> --allowedTools Read,Grep,Glob --tools Read,Grep,Glob --permission-mode dontAsk --output-format json` with `--add-dir <tree>` for routing, validating `structured_output` in the result envelope. Both have a 10-minute timeout, read the prompt from `$PACK_DIR` with absolute `$ARTIFACTS_DIR`/`$PACK_DIR` substituted, and retain logs under check artifacts; stderr receives the log path and a failure tail up to 4000 characters. Tests override `DEVKIT_CODEX` or `DEVKIT_CLAUDE`. Optional `TYPESAFE_API_KEY` enables Jev for judge routing and refine reflection only: routing sends changed-file diffs to TypeSafe; reflection sends new decisions' titles/bodies and citing documents. Without a key nothing is sent there. Keys come only from the environment and are never included in command output or Jev records. `TYPESAFE_API_URL` overrides the test endpoint. Executed fixtures clear the key and exercise relevance selection and both fallbacks with fake CLIs.

`devkit doctor [dir]` prints required, part-specific and optional findings with remedies, and exits 1 only for missing required items. It checks installation in the `Installed providers:` list from `archon doctor` with a 20-second timeout. If that list is absent or the command fails or times out, it checks for an executable regular file at `$ARCHON_HOME/plugins/archon-provider-<provider>` (default `~/.archon/plugins/`). Provider diagnostic failures are shown as warnings without failing the finding. Other external commands retain their three-second timeout. The same report follows every non-dry-run init file summary and never makes init fail. First install then names the configured `devkit-optimize` skill.

The plumbing test checks every YAML/prompt `devkit` call against command registration and rejects script nodes and non-Claude source providers, and checks strict AI schemas and Codex rendering. Go tests cover mechanical decisions and schema contracts. Live AI and external publication are outside fixture coverage. Dependency analysis tools are absent; materials use Git facts, and refine AI nodes receive a complete decision index plus relevant full records. The configured provider supplies AI writers, judges and the CLI fallback; PR publication uses GitHub.

Doctor also reports required **kit and binary**: all commands in installed workflows and skills must be registered, otherwise it lists the missing commands. The separate **kit version** item (needed for a part) compares the stamp's digest to the binary's kit, names both versions on mismatch, or reports no stamp. It recommends reviewing differences with `devkit init`; this item alone never fails doctor.

Delivery starts and `devkit status` check decisions cited by the branch's unit, latest requirements,
and latest plan against the locally available remote base ref (or local base branch). If a cited
decision is superseded there, run `sync` on the same branch, then `requirements` for unit/requirements
citations or `plan` for plan-only citations. Later stages refuse to start; the target and earlier
stages remain available. The target's `returned` input includes old and successor decision text and
citing items: change only affected items, moving citations alone when the choice changes nothing.
Refine owns unit files: when superseding a decision, it retains every citing open unit with successor
citations or withdraws it where allowed; done units remain unchanged. Requirements refuses unit
citations to decisions that are not agreed.
Missing or done units still have their own start refusal. This check never fetches; a missing base
ref is reported as a skipped check.

## Evaluate quality judgments

Run `devkit eval judge [--case <name>]... [--dir <cases folder>] [--out <folder>]
[--standards <folder>]` by hand after editing standards, routing or judge prompts. This calls the
real configured provider CLI and costs real provider calls. It measures prompts and standards;
Archon node wiring is not exercised, and checks are not run.

Cases live under `<eval>/judge/` (`eval` defaults to `.devkit/eval` in `.devkit/config.json`). The
kit source keeps its own cases in `eval/judge/`; init does not install those cases. Each folder has
strictly validated `case.json`, a small `base/` repository and `change.patch` applied with `git apply`.
`case.json` requires `description` and `expected`; each finding has `id`, `quality`, `file`, and a
`summary` explaining the defect and consequence. Optional `not_expected` uses the same shape for
specific false positives. IDs are unique across lists, file paths are repository-relative, and
`expected: []` represents a clean change. Optional `requirements` is verbatim request text passed
as the normal requirements goal and completion condition; there is no plan. All cases in the kit
source's `eval/judge/` include it so scope judges against an agreed request, including the clean change. Without it the case
uses review-only materials. Judges do not receive descriptions or expected findings.

The evaluator builds a base commit plus a committed patch branch and runs the actual `judge prepare`,
`changes`, `route` and `launch` command implementations. It uses the same configured path rules,
Jev and read-only CLI routing fallback as the workflow. Every launched quality gets the installed
`judge/commands/quality.md`, with `$INPUTS.quality` resolved, and the `output_format` read from its
`judge-<quality>` YAML node. If no workflow pack is installed it uses the embedded pack. Standards
come from the explicit flag, installed standards, or embedded standards, in that order. Both
providers use their existing read-only CLI adapters, including schema validation and log files.

One additional read-only CLI call per case matches every reported quality, file, line, failure
scenario and evidence against both lists. Its strict schema requires a reported ID or no match
for every expected and not-expected ID. Stdout and `<out>/result.json` contain found and missed
expectations with summaries, not-expected findings reported, launched versus expected qualities,
routing misses, and case/quality totals. Findings matching neither list are shown as `other` and
are not scored as wrong. The output path defaults to a new printed temporary folder; materials,
answers and logs remain there. Scores do not change exit status: a completed run exits 0, errors
exit non-zero. Continue using `archon workflow test` to validate wiring with stubbed AI.
