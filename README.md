# dev-kit

dev-kit gives Archon a portable development process: the owner decides what matters, a change stays in one place, and the process corrects itself from observed failures. Nine workflows, three session skills, repository-owned standards and the Go binary `devkit` work together. AI sessions write and judge; the binary checks facts, enforces transitions and preserves evidence.

## Install

Target repositories need Git with at least one commit, Archon with the installed and usable provider selected in `.devkit/config.json` (`claude` by default, or `codex`), and `devkit` on `PATH`. Install the provider with `archon provider add claude` or `archon provider add codex` and authenticate it for workflow use. The matching signed-in CLI always selects refine relevance. It also supplies the routing and document-reflection fallback when `TYPESAFE_API_KEY` is unset or Jev cannot answer. Each configured session tool also needs its CLI for skills. Pull requests use an authenticated GitHub CLI (`gh`) and the `origin` remote. Supply the tools required by your repository's checks. Go and mise are needed only to build and test dev-kit itself.

From this checkout:

```sh
mise install go
scripts/install.sh
export PATH="$HOME/.local/bin:$PATH"
cd /path/to/your-repository
devkit init
```

`scripts/install.sh --bin /your/bin` changes the binary destination. `devkit init [--yes | --no] [--dry-run] [--provider claude|codex] [--sessions claude,codex] [--language <code>] [dir]` installs the embedded kit without committing: `.archon/workflows/dev/`, three skill folders for each session: `.claude/skills/` for Claude, `.agents/skills/` for Codex, initial standards at the configured `standards` path (default `.devkit/standards/`), `.devkit/config.json` and `.archon/config.yaml`.

Choose owner-facing prose with `devkit init --language en` (or `--language=pt-BR`). It uses the config's validation: a non-empty string, excluding whitespace-only values. On a fresh install without the flag, init reads the first non-empty locale from `LC_ALL`, `LC_MESSAGES`, then `LANG`, takes the part before `_`, `.` or `@` and lowercases it (`ja_JP.UTF-8` becomes `ja`, `pt_BR` becomes `pt`). `C`, `POSIX` and an empty locale use `en`, the built-in default. Init prints the selected language and its source; `devkit-optimize` can fit it to the repository. Reinstalling preserves the existing language unless `--language` is explicit. Like `--provider`, an explicit language is saved even when config replacement is declined, preserving unrelated keys and formatting.

Init compares all destinations before writing. It lists differing and extra files and asks once per workflow/skill folder: `replace N files and delete M files? [y/N]`. No keeps those files; missing files are still added. Extra files are considered only within kit-owned workflow and skill folders, including a skill's previous folder after a rename. Existing standards are repository-owned: any difference, missing file or extra file prompts once for the whole folder; No leaves it completely untouched. Each differing configuration file has its own question. Identical files produce no file-change lines, and absent files are written without asking.

When workflows match the selected kit (a fresh install, Yes, or no workflow differences), init records `{ "devkit_version": "<version>", "kit_digest": "<digest>" }` in `.archon/workflows/dev/.devkit-installed.json`. No preserves the previous stamp, even when missing workflow files are added. The stamp belongs to the workflow group and is never an extra file. Hand edits do not update it. `devkit version` prints the binary version followed by the embedded kit's SHA-256 digest; identical embedded paths and contents give the same digest across builds.

Every workflow checks its YAML and command files against the running binary's registered commands before recording a start. Within a run it reads the installed pack from `$PACK_DIR`. A missing command ends with `not_started`, names the command, and explains how to align the binary and kit, including the installing version when stamped. Tell the owner and do not retry that run.

Choose the workflow provider and session tools independently:

```sh
devkit init --provider codex
devkit init --provider claude --sessions claude,codex
devkit doctor
```

`provider` defaults to `claude`; absent `sessions` defaults to that provider alone. Explicit sessions are retained when changing providers. Both flags are saved even when config replacement is declined, preserving unrelated keys and formatting, like `--skill`. Source workflows remain Claude YAML. A Codex install sets every AI node to `codex` and removes `allowed_tools`, `denied_tools` and `settingSources`; later changes are ordinary workflow-folder differences with a y/N question. Every AI output schema is strict: all properties required, nested objects closed; inapplicable values use empty strings, empty arrays or `false`. Skill paths, rename comparison and deletion protection apply separately to each session folder. A previous folder whose files devkit cannot identify as its own is never offered for deletion.

Judge prepare keeps one byte-exact judged tree per branch at `$STATE_DIR/trees/<encoded branch>/`, alongside records. `judge/target.json` and quality materials give its absolute `tree` path; judges read it while checks may alter the checkout. The first run exports the full commit. Later runs apply only added, modified, deleted and type/mode changes, preserving executable bits and symlinks and removing empty folders. Commit markers live separately at `$STATE_DIR/trees/.commits/<encoded branch>` so no repository file is reserved. Prepare removes the marker before updating and writes it last; a missing marker, missing commit or failed update triggers a fresh full export. It also prunes trees of deleted local branches, except the branch being judged, on the next prepare after `archon complete` or branch deletion.

Codex workflow judges run with full access: tool restrictions cannot prevent writes. `mutates_checkout: false` makes Archon fail a node that changed the checkout, detecting a write after it happens. The separate Codex CLI fallback uses `--sandbox read-only`; routing runs with `--cd <tree> --skip-git-repo-check` because the exported tree has no Git metadata. Prompt and artifact paths are absolute. Claude fallback uses `-p --json-schema <schema> --allowedTools Read,Grep,Glob --tools Read,Grep,Glob --permission-mode dontAsk --output-format json`; routing adds `--add-dir <tree>` to permit reading the state tree. Its validated answer is `structured_output`, not the plain `result` text. `DEVKIT_CLAUDE` and `DEVKIT_CODEX` override fallback executables for tests.

`devkit doctor [dir]` reports required items (Git and a committed repository, Archon/version, the selected Archon provider, devkit on PATH, installed kit files, config and standards ownership); parts (each session CLI, authenticated gh and origin for PR, and the provider CLI for fallback without a TypeSafe key); and optional items (key presence and configured checks). Missing items explain the blocked feature and remedy. Empty checks name the configured optimize skill. Required failures exit 1; other findings exit 0. It checks installation in the `Installed providers:` list from `archon doctor` with a 20-second timeout. If that list is absent or the command fails or times out, it checks for an executable regular file at `$ARCHON_HOME/plugins/archon-provider-<provider>` (default `~/.archon/plugins/`). Provider diagnostic failures are shown as warnings without failing the finding. Other external commands retain their three-second timeout. Init prints this report after its file summary without failing on findings, then names optimize on the first install.

Doctor's required **kit and binary** item checks all installed workflow and skill commands and lists missing commands. Its separate **kit version** item (needed for a part) compares the recorded digest with this binary's kit; a difference names both versions and recommends `devkit init` to review differences and approve replacements. An absent stamp is reported explicitly. The version item alone never fails doctor, and a digest difference alone never rejects a workflow start.

Choose distinct skill names using lowercase letters, digits and hyphens:

```sh
devkit init --skill devkit-runs=project-runs --skill devkit-discuss=project-discuss --skill devkit-optimize=project-optimize
```

The `skills` mapping in `.devkit/config.json` stores these names for later init runs. Installed folders, front matter, cross-references and watch-script paths use them. Declining config replacement still applies explicit `--skill` flags to only its `skills` value; other keys and their formatting are preserved. The configured standards destination is retained even when replacing config. The names used below are the defaults.

Symbolic links at destinations or their repository ancestors are named and require confirmation. Yes replaces the link itself; its target is untouched. No skips affected destinations. A declined group also prevents replacing a shared ancestor link that would remove its files. `--yes` and `--no` answer every question; without a terminal on stdin, omitted answers default to No with a notice. `--force` is no longer accepted. `--dry-run` lists comparisons and questions without writing or deleting anything. Init ends with the paths written, deleted and left alone; declining changes is successful. Malformed JSON config can be kept or replaced using default destinations for comparison; applying only `skills` requires a JSON object, so a malformed config must be replaced to apply a rename. Invalid destination settings are errors.

Review and commit the installed files before starting runs. A committed base branch is required for workflow execution.

Optionally set `TYPESAFE_API_KEY` in the workflow environment to use Jev for judge routing and refine document reflection only. For `judge` routing, each changed file's diff is sent to TypeSafe's API. For `refine` reflection, new decision titles/bodies and citing documents are sent. Without a key, nothing is sent to TypeSafe and the configured provider CLI performs these checks. Refine relevance always uses the configured provider CLI. The key is read only from the environment and is never recorded. A failed or malformed Jev result also falls back to the configured provider CLI; partial results are discarded. `TYPESAFE_API_URL` overrides the endpoint for tests.

The supplied `.archon/config.yaml` disables bundled default workflows and commands and sets `worktree.baseBranch` to `main`. Incorporate these settings into existing Archon configuration; change the base branch there when necessary. Keep workflow sources committed and unchanged while starting runs.

## Use the flow

```text
devkit-discuss → refine one topic → approval → refine PR → owner merges
  → next pickable topic, or for each settled work unit:
    requirements (unit=<id>) → plan → implement → judge ⇄ fix → pr
    sync → judge when needs_judge → pr
  → retro → approval → correction PR → owner merges
```

After the first `devkit init`, which ends by naming it, run `devkit-optimize` once in a session: it reads the repository, proposes the checks, paths, base branch and worktree preparation, reconciles the installed standards with the repository's own conventions, and writes what the owner approves. `devkit-discuss` compares the owner's words with agreed decisions and prepares a confirmed memo in the session. `devkit-runs` starts and watches workflows and relays their recorded results. Workflows do not invoke one another.

Run `refine` on a `refine/<name>` branch. One run settles one topic: first decisions and next topics, then documents and units for settled areas. Each phase has its own three-round machine-check/judge loop. In the document phase, the machine requires a citation for every new decision; Jev checks reflection at probability >= 0.5, or the configured provider CLI checks it when Jev is unavailable. Missing reflection blocks the round, and the configured workflow provider judges wording. Open points end with `question`; a cleared proposal ends with an approval question. Restart with `approve=<version>` only for the version the owner approved. Approval marks decisions agreed and the current topic settled. Publish the branch through the repository's PR process; merge its refine PR before starting any unit it created. `refine` itself does not publish a PR.

`devkit refine prepare` selects relevant decision bodies once for all decide/document writer and judge rounds. `refine/input.json` contains `decisions`, an index of every decision (`id`, `title`, `status`, `depends_on`, `supersedes`, `path`), and `relevant_decisions`, the relevant full records. The required set includes the topic's transitive decision dependencies and citations, pending and previous proposal decisions, and their supersession chains in both directions, followed to closure. Decisions that open units cite alongside them are added one hop only; documents' co-citations are not followed, because an overview document would pull in everything. Other `agreed` decisions are selected together by one read-only call of the configured provider's CLI. `refine/relevance-input.json` supplies the topic title/body, requests' owner words and supplement, memo, and each candidate's ID, title, body and path; `always_ids` lists required decisions that need not be returned. Jev is not used for relevance because live API measurements did not separate relevant from unrelated decisions; the CLI selected more precisely without measured misses. This broad filter favors inclusion when uncertain, excludes unrelated features of the same system, and considers indirect constraints such as legal and calendar rules. Superseded and withdrawn decisions only enter through the required rules. `refine/relevance.json` records each decision's inclusion and source; if the CLI fails or returns an invalid selection, all agreed decisions are included and the failure reason is recorded. Writers and judges read the relevant set in full, inspect other indexed paths when titles suggest relevance, and read every superseded, depended-on or cited decision in full. Machine checks, materialization, reflection and unit-area checks retain their full checkout data.

Find open topics whose decision dependencies are agreed with `devkit decisions topics`. Pass a selected topic as refine's `topic`. Run `devkit decisions units` (or `--json`) in the base checkout to list units with `id`, `title`, `depends_on`, `state` and `ready`. Start open units marked ready: all their prerequisites are done. Make a separate branch and pass its ID directly:

```sh
archon workflow run requirements --branch feature/filter --detach \
  --input unit=U-a1b2c3
```

Requirements refuses to start a done unit or one waiting for prerequisites. The same recorded request can resume after its successful PR when its valid done marker names that branch. Prepare reads the unit and its cited agreed decisions as constraints. For a direct settled request, use `owner_words` instead; keep session findings in `supplement`. Every delivery request goes through `plan`, even when its decision list is empty.

The delivery owner questions belong in requirements and plan. Implementation, code judgment and fixes return `back` to plan for newly discovered owner decisions. PR asks only about configured `ask` changes not already covered by an owner-decided row's `touches`. `pr` judges the body, commits review backlog and the branch unit's `<units>/<unit-id>.done.json` marker together, pushes and opens or updates a GitHub PR; the owner decides merging. `sync` rebases and returns `needs_judge` after conflict resolution.

Small changes (a typo, a clear small bug, a test or configuration fix) do not go through the workflows: the session edits directly, following the standards, and runs `devkit checks`, which works outside a run too. The workflows are for owner decisions, changes of structure, data, contracts, security or cost, and unsettled topics. `devkit status` shows, for every branch with a recorded run, its unit ID from the request record, the last outcome, the unanswered questions and the next instruction; a session uses it to pick up open requests. A question ends its run. Relay its exact ID and text; restart the named `next_workflows` workflow with the owner's verbatim `answers` JSON. Follow `back.to` for a step back. A stop or rejected start is not a pass. Restart and fix limits default to 3; only the owner's words in `continue` authorize a start that reaches the restart limit. The installed `devkit-runs` skill gives the handoff procedure.

Each branch in `devkit status --json` includes an `errors` array of strings alongside the existing fields. An unreadable answer adds its question ID, answer file path and failure reason; the array is empty when no answer read failed. Text output shows these failures under the branch as `unreadable: ...`. An unreadable answer is not listed as an ordinary unanswered question. The `next` instruction asks you to resolve unreadable answers and check status again, before following any supersession-related sync instruction. Status does not change the records.

When the last result is a passing `judge` with `inputs.requirements_version: null`, `next` says the review is complete and asks you to relay the report to the owner. A passing judge with requirements directs you to start `pr`.

## After the pull request

Read comments and checks with `gh pr view <branch> --comments` and `gh pr checks <branch>`.
Pass in-scope corrections or CI failures verbatim, including their source, to `fix` as `review`
(for example `Review comment by <login> on path:line: ...` or `CI job <name> failed: <log excerpt>`).
Review fixes require passed requirements and plan and the same judged change as HEAD; the PR's
backlog/done-marker commit is accepted. A passing fix needs `judge` again, then `pr` updates the PR.
Review findings are saved in the next backlog with source `review`.

For a defect discovered after merge, including one repaired directly outside the workflows, write
the owner's description to a file and record it in the current checkout:

```sh
devkit backlog escape --defect-file /path/to/defect.txt --entered <commit-or-PR> [--fixed <commit-or-PR>]
```

`--entered` and optional `--fixed` accept a locally resolvable commit, a commit hash, a GitHub PR
URL or a positive PR number. Local commits are saved as full hashes. Defect text is accepted only
through `--defect-file`. The command prints `.devkit/backlog/escapes/<random-hex>.json`; commit it
with the fix or through the repository's PR process so it reaches the base branch.
Retro traces the entering change and missed stage, including routing sentences in `concerns.json`.
For every used escape a judge should have caught it proposes a minimal case under `<eval>/judge/`
(`eval/judge/` in this repository, with `eval` configured accordingly). After machine checks and
independent judgment, it runs those cases against a temporary copy of the proposed standards and
concern map. The approval question reports found/missed expectations and provider failures in the
configured language; misses and failures do not block approval. Only escape cases incur these
provider calls. Approval applies the cases and corrections and deletes the consumed escape records
in the same commit.

Comments that change or add to the request go to `requirements` on the same branch as `owner_words`,
then through the usual flow. Fix returns out-of-scope or disputed review items as `outside_requirements`
with reasons and `back` to requirements. Never answer or resolve comments on the reviewer's behalf.
For CI failures, suggest `devkit-optimize` to add the job as a configured pre-PR check.

For a branch whose last result is a passing PR, `devkit status` uses `gh` when available to show its
state, URL, review decision and failing checks. The lookup has a 15-second timeout; failures are shown
without failing status. `next` routes failing checks or requested changes to review fix/requirements,
merged PRs to `archon complete <branch>` and closed PRs to “closed without merging”.
After merging, `archon complete <branch>` removes the worktree and local branch and keeps the remote.
Periodically run `archon isolation cleanup --merged`.

`devkit checks` compares only failing checks with the base branch's merge-base commit in a temporary detached worktree. It uses the same commands, timeouts and exit codes, after running the repository's `.archon/config.yaml` `worktree.postCreate` commands in order (each uses `postCreateTimeout` in milliseconds, default 300000). Preparation failure makes the comparison unavailable; the worktree is removed afterward. Passing checks and environment failures (75 after its existing retry, or timeouts) are not compared. `--no-base` skips comparison; skeleton checks use it. In implement and fix (`--base-on-repeat`) a check is compared only when it also failed in the previous pass, so a first failure, the usual case while writing code, costs nothing extra; judge and sync compare every failing check. A check not compared records `base: not_compared`.

Each failing result records `base: failed|passed|unavailable` and a `base_log` when run; the gate lists `preexisting` and `regressed` checks. If every failure is preexisting, implement/fix stop immediately without consuming a pass or invoking step-back. Judge reports a preexisting check as failing on the base too and stops instead of directing the branch to fix. Tell the owner which checks fail and their log paths: fix the base branch first, then restart the same workflow. That restart is `retry_after_base`, exempt from `limits.restarts`; `devkit status` says so. Retrying before the base is fixed is pointless. Regressed or unavailable comparisons retain the existing implement/fix failure behavior.

## Why the parts fit together

`<standards>/concerns.json` maps ten qualities—structure, data, contract, security, ui, cost, test, delivery, scope and readability—to their principles and phase criteria. Every numbered principle has exactly one owner, counting `every_quality` as one shared assignment. Every review document except `review/README.md` must be mapped. `devkit concerns check` validates this and prepare stops on violations. The machine derives writer/judge materials from this map. Common judgment rules live in `review/README.md`; owner authority lives in `authority.md`.

`devkit judge launch` records the selected qualities in `judge/launched.json` from the same decision as its `launch_<quality>` flags; `devkit eval judge` uses the same launch processing. Code quality judges read this file and the fixed `concerns.json`: they report failures of their own principles and may report another quality's failure only when its owner was not launched. They receive no `every_quality` principles; these remain in requirements, plan, implement and fix materials. The machine checks `decision_paths` changes absent from plan rows, and scope checks authorization under `working.md#6`. A consequence that fails their own quality remains reportable, such as a missing test or a changed caller contract, even when another judge reports the defect itself.

Each quality has one `review/<quality>.md` with Decisions, Code, Blocking and Not reported sections. Plan judges choices and the skeleton; code judges bodies and conformance to passed choices. Agreed choices are carried by reference rather than judged again.

Decision records `<decisions>/D-<id>.md` hold repository decisions. IDs derive from branch and draft key using six lowercase hexadecimal characters, extending on collision; units and topics use the same scheme with `U-` and `T-`. Re-proposals retain kept keys' IDs and remove withdrawn proposed files. Agreed records are replaced through `supersedes`. Dependency traversal identifies affected decisions, documents and units. Passed owner/principle plan rows also become agreed decision records; provisional rows remain in the plan and PR body.

Plan settles structure before detail. New structure rows explain kind, reasons for change, volatility and callers with evidence or `unknown`. After structure passes, detail may change signatures but cannot add, remove or move files. Code judgment receives uncovered changed files, folder spread and ancestor siblings. Structural defects on the change's path block even if pre-existing; defects elsewhere are deferred. Jev selects qualities from each file's diff, using each quality's `routing` description and `routing_threshold` in `concerns.json`; when unavailable, `devkit judge route` runs the configured provider CLI read-only with schema validation. Configured globs and plan rows also select judges. Scope always runs; source/test changes also select test and readability. Structure routing covers function responsibilities, repeated checks, bypassed guarantees, caller state or call order, behaviour flags and values passed through layers as well as module placement. Data routing covers valid states and values and domain rules expressed as branches as well as stored data. Readability owns names, comments, conventions and shared rule definitions under `engineering.md#8`. For every added function, conversion, check or rule, it searches the whole judged tree named in `judge/target.json` for existing behaviour that must change together. Re-implementing such behaviour blocks; similar text with independent reasons for change or contracts does not. Placement and dependencies remain structure's responsibility.

Before pushing, PR commits `.devkit/backlog/<encoded-branch>.json`: deferred and carried findings, mechanical-check candidates and accepted objections, with their source judgments. The current branch's valid backlog file and the valid done marker for its request unit naming that branch are excluded from judged-change identity and decision detection. Other files under the units folder remain part of the change. The marker contains `unit` and `branch`, is validated against the embedded `unit-done.json` schema, and is kept unchanged on reruns. A branch without a unit writes no marker. A unit becomes done in the base checkout when its PR merges. `devkit decisions check` reports invalid markers and markers for missing units. Refine and retro preserve done units and cut new units for follow-up work. Identity otherwise preserves file names and every added/removed line, including whitespace, while ignoring hunk positions. `retro`, on a `retro/<name>` branch from the base, proposes work units, check additions and standard edits from the backlog. Its machine gate and independent judge precede an approval question. Approved changes and deletion of consumed backlog files share one commit.

## Repository settings

Edit `.devkit/config.json`; `devkit config` prints effective settings. The authoritative defaults and validation are in `internal/config`, and installed values are in `kit/.devkit/config.json`.

- `provider`: `claude` (default) or `codex`; selects installed workflow AI, refine relevance selection and the Jev fallback. Run init again to render changed workflows.
- `sessions`: distinct session tools (`claude`, `codex`); absent defaults to the provider alone; `[]` installs no session skills.
- `language`: owner-facing prose; built-in default `en`. Fresh init selects it from `--language` or the environment locale.
- `skills`: kit identifiers `devkit-runs`, `devkit-discuss` and `devkit-optimize` mapped to distinct installed names; defaults match those identifiers. Entries merge individually with defaults.
- `standards`, `docs`, `decisions`, `units`, `topics`, `eval`: repository-relative folders; default `.devkit/standards`, `docs`, `.devkit/decisions`, `.devkit/units`, `.devkit/topics`, `.devkit/eval`.
- `checks`: named shell commands with `command`, `timeout_seconds` and optional `paths`. No checks are installed by default; add the repository's real checks. Omitted paths run every time; an empty paths array selects none. Exit 0 passes, 75 retries once as an environment error, 77 means not run, and other exits fail. Timeouts are environment errors.
- `skeleton_check`: names of checks to run on the skeleton; empty by default.
- `qualities`: path globs for the fixed qualities. Routing and plan rows also select judges; the concern map controls their plan/code phases.
- `decision_paths`: `{paths, quality, ask}` detection rules. Defaults include dependencies, migrations, infrastructure, CI, environment examples, workflows, standards and `.devkit/config.json`. Before/after check configurations are combined so deleting a check does not bypass it.
- `limits.restarts`, `limits.fixes`: default 3 each. `limits.passes`: passes `implement` gives each release step, default 3; a step that uses them up stops the run with the step and its failing checks, independent of how many steps the plan has.

Missing settings use defaults; quality and limit entries merge individually, while other supplied arrays replace defaults. Globs support `**`, `*` and `?`. Repository-specific facts and conventions belong in these settings and standards.

## Evaluate the real judge

Fixtures test workflow execution with stubbed AI. Run this evaluation by hand after changing
standards, routing or judge prompts to measure whether the real quality judges find known defects:

```sh
devkit eval judge
devkit eval judge --case duplicate-price-helper --case clean-local-rename \
  --dir eval/judge --out .local/judge-eval --standards kit/.devkit/standards
```

The first command uses `<eval>/judge/`, where `eval` defaults to `.devkit/eval` in
`.devkit/config.json`. In this source checkout, use `--dir eval/judge` for the kit's own cases;
`devkit init` does not install them. `--case` selects folder names and can repeat; omit it to run
all cases. `--dir` names the folder containing case folders. `--out` defaults to a new temporary
folder, whose path is printed. Standards come from `--standards`, then the repository's installed
standards, then the binary's embedded standards. Relative flag paths are relative to the current
directory. Installed workflow prompts and YAML take precedence over embedded ones.

This costs real calls to the configured provider's signed-in CLI. Each case uses the real judge
prepare, changes, route and launch steps on a temporary Git repository, then one read-only CLI call
per launched quality using `judge/commands/quality.md` and the output schema read from its
`judge-<quality>` YAML node. Routing uses Jev when `TYPESAFE_API_KEY` is set, with the workflow's
same CLI and all-quality fallbacks. One additional read-only provider call matches all findings
against the case's expected and not-expected lists. Checks are not run, and the judges are given no configured checks, so a defect a check would catch is not left to it: the evaluation measures what the judges find themselves. Archon node wiring is not
exercised; keep running the workflow fixtures for that.

Each case folder contains `case.json`, `base/` and `change.patch` (a patch that `git apply` can apply
to `base/`). Keep enough unchanged context to prove the defect, including existing helpers for
duplication cases. Base files must be regular files, without Git metadata. For example:

```json
{
  "description": "A settings read error becomes success",
  "requirements": "Preserve settings read failures so callers can distinguish an outage from empty settings.",
  "expected": [
    {
      "id": "settings-error",
      "quality": "contract",
      "file": "app/settings.py",
      "summary": "Returning an empty object on a storage error hides an outage from callers."
    }
  ]
}
```

`expected` is required and may be `[]` for a clean change. Optional `not_expected` has the same
finding shape and describes a specific false positive a correct judge must avoid. IDs must be
unique across both lists; qualities must have a code judge (`delivery` is plan-only), files must
be repository-relative and exist in the base or changed repository, and summaries must explain
what is wrong and why it matters. Unknown properties, missing fields and invalid types are errors.
Optional non-empty `requirements` supplies the verbatim request; all cases shipped in `eval/judge/`
include it so scope judges against an agreed request, including the clean change. It becomes
normal `requirements.json` goal and completion-condition material, without a plan. Other cases
use review-only materials with neither requirements nor plan. Descriptions and expectations are
not supplied to the quality judges.

Stdout and `result.json` report expected findings found or missed with summaries, forbidden
findings reported, launched versus expected qualities, and case/quality totals. A missed finding
whose quality was not launched is marked as a routing miss. Unmatched reported findings are
listed as `other`, not scored as wrong: they may be real defects beyond the case's known list.
The output folder retains fixed standards, prompts, per-quality answers, matching input/output and
CLI logs for inspection. A completed evaluation exits 0 regardless of scores. A provider timeout
or malformed answer in one case does not stop the run: that case is recorded as not scored, listed
at the end and left out of the totals, and the other cases continue. Each quality judge gets 30
minutes. Only errors that prevent the run, such as invalid case files, exit non-zero. No real provider is called by the tests.

## Test the kit

From this source checkout:

```sh
mise exec -- go build ./...
mise exec -- go vet ./...
mise exec -- go test ./...
scripts/test-kit.sh
```

`scripts/test-kit.sh [workflow ...]` builds both executables, initializes fresh Claude and Codex repositories under `.local/tmp/`, requires workflow validation without warnings, checks principle ownership, tests fresh installations, then copies source-only executed fixtures and tests them for both providers. Its isolated Archon home uses `$HOME/.archon/plugins`, overridable with `KIT_ARCHON_PLUGINS`. Every workflow has an executed main path: bash runs for real and AI is stubbed. PR publication is stubbed; sync exercises a real local rebase conflict. `cmd/devkit-fixture` supplies test-only setup and file writes and is absent from the shipped binary. The plumbing test rejects YAML/prompt calls to unregistered commands. Go tests cover the evaluation with fake CLIs; `node --test` runs the CLI smoke tests.

See the [workflow guide](kit/.archon/workflows/dev/README.md) for artifacts and execution contracts, and [design rationale](docs/design.md) in Japanese. Update these explanations alongside their authoritative code, YAML, schemas and standards.

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


## Current limits

There are no dependency facts from analysis tools; current materials use Git facts. Refine gives AI nodes a complete decision index and relevant full records. Source AI nodes use Claude; installed workflows and the read-only relevance selection and routing/reflection fallback use the configured provider. Codex workflow judges cannot be prevented from writing; checkout mutations are detected by Archon. If both are unavailable, routing selects every routable quality and reflection fails. Pull request publication supports GitHub through `gh`. Live AI behavior and external PR publication are outside fixture coverage.
