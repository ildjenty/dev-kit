# Development workflows

The owner decides what matters, changes stay local, and observed failures improve the process. Archon runs nine workflows; `devkit` performs mechanical checks and records transitions. The installed `dev-discuss` and `dev-runs` session skills handle conversation, starting runs and relaying results. Workflows never start each other.

## Sequence and handoffs

Use `dev-discuss` to produce an owner-confirmed memo, then `refine` on a `refine/<name>` branch. Refine settles one topic through decide and document phases, each with at most three rounds. Pass `topic=T-<id>` for an open topic listed by `devkit decisions topics`; an empty topic starts from the request itself. After the owner approves the recorded version using `approve=<version>`, publish and merge the refine PR through the repository's PR process. Refine commits its proposal and agreement but does not publish a PR.

Only after that merge, start each eligible unit on its own branch: `requirements` with `unit=U-<id>` → `plan` → `implement` → `judge` ⇄ `fix` → `pr`. Honor unit dependencies. Direct settled requests use requirements' `owner_words`; session observations go in `supplement`. Plan is required for delivery. Review-only `judge` also works without requirements or a plan. Use `sync` to rebase; run judge when its result says `needs_judge`, then return to PR.

After backlog reaches the base branch, run `retro` on a `retro/<name>` branch created from that base. It proposes units, check additions, standard edits or reasoned dismissals; its approval applies the changes and deletes consumed backlog files in one commit. Publish and merge the correction PR before starting its units.

A `question` result ends the run. Relay every question and ID and restart its named `next_workflows` workflow with verbatim owner answers. Refine and retro approval use the exact recorded version. Requirements and plan ask unresolved request/design questions; implementation, judge and fix return `back` to plan for owner decisions. PR asks only for uncovered `ask` changes. Follow `back.to`; `stop` describes unresolved work, and `not_started` means prerequisites rejected the start. `carry` is a plan result carrying detail findings forward. Counted restarts and fix runs obey configured limits; supply owner words as `continue` only when the restart limit is reached.

## Concerns, decisions and structure

The code quality set is `internal/concerns`; the repository-owned `<standards>/concerns.json` assigns principles and plan/code criteria to structure, data, contract, security, ui, cost, test, delivery, scope and readability. Every numbered standard principle has exactly one assignment: one quality, `every_quality`, or a named writer/non-quality judge in `others`. Every named document exists and every review document except `review/README.md` is mapped. `devkit concerns check` validates the map, including during plan, judge, refine and retro prepare on fixed criteria.

Materials come from that map. `review/README.md` owns common judgment rules; `authority.md` owns owner decision boundaries. Commands specify inputs, writes and output contracts and refer to standards for rules. Quality criteria separate Decisions from Code, Blocking and Not reported. A choice passed in plan is checked for conformance in code rather than chosen again.

Repository decisions live in `<decisions>/D-<id>.md`, units in `<units>/U-<id>.md` and topics in `<topics>/T-<id>.md`. IDs derive from branch and draft key, starting with six lowercase hexadecimal characters and extending on collision. Re-proposals keep retained keys' IDs and remove withdrawn unapproved files. Dependency traversal reaches affected decisions and citing documents/units. Topics are pickable only when open and all decision dependencies are agreed. Requirements prepare loads the selected unit and agreed records as constraints. Plan rows apply them with `ref`; passed new owner/principle rows become agreed records in the skeleton commit. Provisional choices remain in the plan and PR body.

Structure comes first in plan. Nature evidence explains placement; its fresh structure judge must pass before detail adds other rows, release units and tests. Each phase has its own three-round loop. Detail can alter signatures but changing file membership or structure rows stops the run. Prepare writes the embedded plan draft schema and its reference dependency to artifacts. Writers cite facts in row evidence.

Judge computes `outside_plan`, folder spread and siblings on ancestor paths. Structural defects on the change's path block, including existing defects; those elsewhere are deferred. Fresh read-only route selects qualities with changed-file evidence. The machine unions route, configured globs and plan rows; scope always runs, and source/test changes select test and readability. AI judges use Claude, fresh contexts and read-only tools on fixed criteria.

## Commands and artifacts

`devkit help` is the command inventory; `cmd/devkit` registers the mechanical packages. YAML passes Archon's `STATE_DIR`, `WORKFLOW_ID`, `BASE_BRANCH`, `ARTIFACTS_DIR`, declared `INPUTS_*` and producer outputs through environment variables. Commands return JSON on stdout, errors on stderr with a nonzero status; `record dir`, help and version return text.

- Refine: `devkit refine start`, `prepare`, `check`, `round`, `decided`, `finish`, `approve`, `not-started`. Artifacts under `refine/` include input, schemas, draft, decided snapshot, phase rounds and approval text. The document phase freezes decide output; scope checks inspect both index and worktree. Approval checks the started run and recorded answer before any repository update.
- Requirements: `devkit record start requirements`, then `devkit requirements prepare`, `round`, `finish`, `not-started`. Input and round artifacts are under `requirements/`; the writer returns requirements as structured output. Failed or unavailable criteria cannot pass.
- Plan: `devkit record start plan`, then `devkit plan prepare`, `facts`, `check`, `settle`, `advance`, `finish`, `not-started`. `plan/input.json`, `draft.schema.json`, `facts-before.json`, `draft.json`, `findings.json` and `structure.json` carry the two phases. Answers bind to requirements version and row content. Allowed provisional implementation updates record `derived_from` the accepted plan.
- Implement: `devkit record start implement`, then `devkit implement prepare`, `next`, `tests-written`, `end`, `state`, `finish`, and `devkit checks`. Separate sessions write tests and code per release unit. Repeated check failure invokes step-back. The workflow commits completed units; new owner choices go back to plan.
- Judge: `devkit record start judge`, then `devkit judge prepare`, `changes`, `launch`, `completion`, `collect`, `finish`, and `devkit checks`. `judge/` contains target, committed source snapshots, diff, decisions, locality facts, answers and per-quality materials. Without a plan, decision changes become `without_plan` materials.
- Fix: `devkit record start fix`, then `devkit fix prepare`, `proceed`, `objections`, `dispute-end`, `round-end`, `state`, `finish`, and `devkit checks`. An independent judge settles evidence-backed objections. Finding references include the original judge run, judge and local ID; acceptance cannot suppress another run's finding.
- PR: `devkit record start pr`, then `devkit pr answers`, `facts`, `assemble`, `judgment`, `state`, `open`, `finish`, and `output`. `pr/` holds facts, machine lists, body and title, report findings and publication result. Publication pushes to `origin` and uses `gh`; it does not merge.
- Sync: `devkit sync start`, `continue`, `state`, `finish`, `output`, and `devkit checks`. Before rebasing, sync writes mapped materials from the request's fixed standards into `sync/input.json`; conflict resolution receives them through `sync/conflict.json`. Failed resolution aborts the rebase and restores the original commit; successful conflict resolution sets `needs_judge`. Sync does not push.
- Retro: `devkit retro start`, `prepare`, `check`, `round`, `finish`, `approve`, `empty`, `not-started`. `retro/` contains backlog input, schemas, draft, check, rounds and approval text. Proposed config/standard edits are restored until approval; an empty backlog passes without approval.

Shared commands include `devkit config`, `devkit criteria`, `devkit concerns check`, `devkit decisions list`, `topics`, `check`, `affected`, `dependents`, `next-id`, and `devkit record start`, `write`, `latest`, `dir`, `limit`, `result`. Consult help for arguments.

## Records and judged identity

Runtime records stay outside Git at `$STATE_DIR/records/<encoded-branch>/`; `feature/filter` encodes as `feature%2Ffilter`. Resolve the path with `devkit record dir` in the run environment. AI nodes receive selected artifacts rather than this directory.

`request/` preserves owner words, supplement and the initial criteria commit. `proposals/`, `requirements/` and `plans/` hold immutable versions; `judgments/` holds targets, findings and evidence. `runs/` records successful starts and restart accounting; `results/` records outcomes and input versions. `questions/` and `answers/` use question IDs, `allowed/` holds PR permissions, and `continuations/` holds owner authorization past restart limits. Every write validates against embedded schemas in `internal/record/schemas/`. Failed starts save no answers; starting an earlier stage invalidates later results.

Passing judgments cover HEAD or an identical merge-base change: file names and every added/removed line including whitespace match, hunk positions are ignored. `.devkit/backlog/**` is excluded, so PR's backlog commit preserves judged identity. Code judgments store `change_id`.

## Backlog contract

Before pushing, PR commits `.devkit/backlog/<encoded-branch>.json` with a `chore:` subject. Its four arrays are `deferred`, `carried`, `mechanical`, `accepted_objections`. Each entry has `finding` (the original recorded object) and `source` (`run_id`, one-based `judgment` record number, `judge`, `kind`, full `target`). Accepted objections are nonblocking dispute findings with the original finding reference. An entry may occur in multiple arrays. Retro input IDs are `<backlog-path>#<category>/<zero-based-index>` and preserve this content and provenance. Consumed files are removed only with approved corrections.

## Configuration and verification

Repository-specific behavior belongs in `.devkit/config.json` and standards. Settings are `language`, `standards`, `docs`, `decisions`, `units`, `topics`, `checks`, `skeleton_check`, `qualities`, `decision_paths` and `limits`. Use `devkit config` for effective values. Defaults install no checks. Check commands decide mechanical outcomes; exit 75 retries once as an environment error, 77 skips, 0 passes, and other exits fail. Both merge-base and current configurations supply checks so removal cannot bypass verification. Config itself is a default `ask` decision path.

For changes to this pack, edit the relevant YAML, prompts and Go decision points; cover changed outcome paths with stub fixtures and behavior with Go tests. Run from the source checkout:

```sh
mise exec -- go build ./...
mise exec -- go vet ./...
mise exec -- go test ./...
scripts/test-kit.sh
```

The script initializes a temporary repository and isolated Archon home, runs `archon validate workflows`, `devkit concerns check`, and `archon workflow test` on the fresh installation. It then copies `test/fixtures/` from source and reruns fixtures with real bash and stubbed AI through a temporary wrapper. `cmd/devkit-fixture` supplies prerequisite records and author file writes and verifies persisted results before scratch cleanup. Test helpers and executed fixtures are absent from `devkit` and the installed pack. PR publication is stubbed; sync executes a real local conflict. `scripts/test-kit.sh [workflow ...]` narrows the executed-fixture selection. Provider plugins default to `$HOME/.archon/plugins` with `KIT_ARCHON_PLUGINS` as override.

The plumbing test checks every YAML/prompt `devkit` call against command registration. Go tests cover mechanical decisions and schema contracts. Live AI and external publication are outside fixture coverage. Dependency analysis tools and Jev-style decision narrowing are absent; materials use Git facts and full records. Claude is the only AI provider; PR publication uses GitHub.
