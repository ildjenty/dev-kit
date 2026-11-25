# dev-kit

dev-kit gives Archon a portable development process: the owner decides what matters, a change stays in one place, and the process corrects itself from observed failures. Ten workflows, three session skills, repository-owned standards and the Go binary `devkit` work together. AI sessions write and judge; the binary checks facts, enforces transitions and preserves evidence. [Workflow diagrams](docs/workflows.md) show every workflow's nodes, which run on AI and which on the machine, and their loops.

## Install

Target repositories need Git with at least one commit, Archon with the installed and usable provider selected in `.devkit/config.json` (`claude` by default, or `codex`), and `devkit` on `PATH`. Install the provider with `archon provider add claude` or `archon provider add codex` and authenticate it for workflow use. The matching signed-in CLI supplies the judge routing fallback when `TYPESAFE_API_KEY` is unset or Jev cannot answer. Each configured session tool also needs its CLI for skills. Issue operations and pull requests use an authenticated GitHub CLI (`gh`); PR publishing also needs `origin`. The repository must support GitHub Issues, sub-issues and issue dependencies, and the configured labels must exist. Use gh with `gh issue view --json parent` support. Enable missing capabilities, update gh or create labels before using the Issue-based flow. Supply the tools required by your repository's checks. Go and mise are needed only to build and test dev-kit itself.

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

`provider` defaults to `claude`; absent `sessions` defaults to both `claude` and `codex`, so skills go to `.claude/skills/` and `.agents/skills/`. Explicit sessions are retained when changing providers. Both flags are saved even when config replacement is declined, preserving unrelated keys and formatting, like `--skill`. Source workflows remain Claude YAML. A Codex install sets every AI node to `codex` and removes `allowed_tools`, `denied_tools` and `settingSources`; later changes are ordinary workflow-folder differences with a y/N question. Every AI output schema is strict: all properties required, nested objects closed; inapplicable values use empty strings, empty arrays or `false`. Skill paths, rename comparison and deletion protection apply separately to each session folder. A previous folder whose files devkit cannot identify as its own is never offered for deletion.

Judge prepare keeps one byte-exact judged tree per branch at `$STATE_DIR/trees/<encoded branch>/`, alongside records. `judge/target.json` and quality materials give its absolute `tree` path; judges read it while checks may alter the checkout. The first run exports the full commit. Later runs apply only added, modified, deleted and type/mode changes, preserving executable bits and symlinks and removing empty folders. Commit markers live separately at `$STATE_DIR/trees/.commits/<encoded branch>` so no repository file is reserved. Prepare removes the marker before updating and writes it last; a missing marker, missing commit or failed update triggers a fresh full export. It also prunes trees of deleted local branches, except the branch being judged, on the next prepare after `archon complete` or branch deletion.

Codex workflow judges run with full access: tool restrictions cannot prevent writes. `mutates_checkout: false` makes Archon fail a node that changed the checkout, detecting a write after it happens. The separate Codex CLI fallback uses `--sandbox read-only`; routing runs with `--cd <tree> --skip-git-repo-check` because the exported tree has no Git metadata. Prompt and artifact paths are absolute. Claude fallback uses `-p --json-schema <schema> --allowedTools Read,Grep,Glob --tools Read,Grep,Glob --permission-mode dontAsk --output-format json`; routing adds `--add-dir <tree>` to permit reading the state tree. Its validated answer is `structured_output`, not the plain `result` text. `DEVKIT_CLAUDE` and `DEVKIT_CODEX` override fallback executables for tests.

`devkit doctor [dir]` reports required items (Git and a committed repository, Archon/version, the selected Archon provider, devkit on PATH, installed kit files, config and standards ownership); parts (each session CLI, authenticated gh for Issues and PRs, origin for PRs, and the provider CLI for fallback without a TypeSafe key); and optional items (key presence and configured checks). Missing items explain the blocked feature and remedy. Empty checks name the configured optimize skill. Required failures exit 1; other findings exit 0. It checks installation in the `Installed providers:` list from `archon doctor` with a 20-second timeout. If that list is absent or the command fails or times out, it checks for an executable regular file at `$ARCHON_HOME/plugins/archon-provider-<provider>` (default `~/.archon/plugins/`). Provider diagnostic failures are shown as warnings without failing the finding. Other external commands retain their three-second timeout. Init prints this report after its file summary without failing on findings, then names optimize on the first install.

Doctor's required **kit and binary** item checks all installed workflow and skill commands and lists missing commands. Its separate **kit version** item (needed for a part) compares the recorded digest with this binary's kit; a difference names both versions and recommends `devkit init` to review differences and approve replacements. An absent stamp is reported explicitly. The version item alone never fails doctor, and a digest difference alone never rejects a workflow start.

Choose distinct skill names using lowercase letters, digits and hyphens:

```sh
devkit init --skill devkit-runs=project-runs --skill devkit-discuss=project-discuss --skill devkit-optimize=project-optimize
```

The `skills` mapping in `.devkit/config.json` stores these names for later init runs. Installed folders, front matter, cross-references and watch-script paths use them. Declining config replacement still applies explicit `--skill` flags to only its `skills` value; other keys and their formatting are preserved. The configured standards destination is retained even when replacing config. The names used below are the defaults.

When `--provider`, `--sessions`, `--language` or `--skill` is explicit, init rejects an existing `.devkit/config.json` object containing duplicate keys before asking questions, writing or deleting files. It checks the entire object recursively, including unrelated settings and objects inside arrays, and compares names after decoding JSON escapes. The error names the duplicated key and asks the owner to remove it. This also applies with `--yes` and `--dry-run`; rejection leaves existing files and formatting unchanged. Full replacement of config that cannot be parsed as a JSON object retains its existing behavior.

Symbolic links at destinations or their repository ancestors are named and require confirmation. Yes replaces the link itself; its target is untouched. No skips affected destinations. A declined group also prevents replacing a shared ancestor link that would remove its files. `--yes` and `--no` answer every question; without a terminal on stdin, omitted answers default to No with a notice. `--force` is no longer accepted. `--dry-run` lists comparisons and questions without writing or deleting anything. Init ends with the paths written, deleted and left alone; declining changes is successful. Malformed JSON config can be kept or replaced using default destinations for comparison; applying only `skills` requires a JSON object, so a malformed config must be replaced to apply a rename. Invalid destination settings are errors.

Review and commit the installed files before starting runs. A committed base branch is required for workflow execution.

Optionally set `TYPESAFE_API_KEY` in the workflow environment to use Jev for judge routing. Each changed file's diff is sent to TypeSafe's API. Without a key, nothing is sent to TypeSafe and the configured provider CLI performs routing. The key is read only from the environment and is never recorded. Failed or malformed Jev results fall back to the CLI; partial results are discarded. `TYPESAFE_API_URL` overrides the endpoint for tests.

The supplied `.archon/config.yaml` disables bundled default workflows and commands and sets `worktree.baseBranch` to `main`. Incorporate these settings into existing Archon configuration; change the base branch there when necessary. Every consumer uses the same resolver: Archon's `BASE_BRANCH`, then `worktree.baseBranch` in the repository's `.archon/config.yaml`, then the origin default from `git symbolic-ref refs/remotes/origin/HEAD`. A missing file or absent setting advances to origin; an unreadable or malformed existing file fails with its path and cause. If none supplies a branch, the error names all three sources; `main` is never guessed. Successful starts, judge targets, checks gate output and sync state retain `base_branch` and `base_branch_source`. Keep workflow sources committed and unchanged while starting runs.

Commands that consume run artifacts require `ARTIFACTS_DIR` before reading or writing. Provider CLI calls also require `PACK_DIR`; a missing required folder setting names the variable and stops instead of using checkout-relative files. Standalone `devkit checks` retains its temporary output folder when no artifacts folder is supplied.

## Use the flow

Each workflow's nodes are drawn in [docs/workflows.md](docs/workflows.md).

```text
devkit-discuss mode (discussion → refine → approval → Issues) → refine PR → owner merges
  → next ready topic, or for each settled work unit:
    requirements (issue=<number>) → plan → implement → judge ⇄ fix → pr
    sync → judge when needs_judge → pr
  → retro → approval → correction PR → owner merges
```

After the first `devkit init`, run the `devkit-optimize` skill with the owner's instructions. It delegates run mechanics to `devkit-runs` and starts the `optimize` workflow; it relays reviewed questions and exact-version approval, then reports the PR. `devkit-discuss` stays active from invocation until settled work and open questions have become Issues. It keeps a running conversation memo of decided, overturned (with decision IDs) and still-open points, updating it as choices change and restating it after interruptions or summarisation. It handles unrelated requests and then resumes. Comparing the decision log, confirming the memo, starting refine and relaying its questions and exact-version approval all belong to this mode. `devkit-runs` starts and watches workflows and relays their recorded results. Workflows do not invoke one another.


`optimize` runs on an `optimize/<name>` branch with `instructions` (verbatim, possibly empty),
`answers`, `approve` and `continue`. Empty instructions request a full repository fit.
A writer reads CI, task runners, layout, guides, conventions and people's language, then drafts
settings, worktree preparation, reconciled standards, routing sentences and short agent instruction
sections. Workflow edits, including per-node provider/model/effort, require the owner's request.
Security, data-loss and authority blocking cannot be weakened. The same writer triages every open
Issue without the configured unit/topic labels: unit with entailed completion conditions, topic
when requirements are unclear, or exclusion with a reason. Related inseparable questions follow
the topic rule in `review/purpose.md`. New topic groups go under an existing topic parent;
units cut from topics are sub-issues, and blocked-by expresses order and waiting.

Each draft is machine-checked and committed as a new proposal version before fresh read-only
judgment against the fixed standards, `harness.md`, `review/harness.md`, `authority.md` and
`review/purpose.md`. At most three rounds run. Checks cover config, concern ownership, edit scope,
title limits, existing open Issue references and topic/blocked-by structure. Workflow validation
uses `archon validate workflows` without warnings; if Archon is unavailable, it explicitly reports
the devkit command-plumbing and YAML parsing fallback. Every configured check runs on the base
in a temporary worktree prepared with the proposed `postCreate`, regardless of path selection.
Failures, skips and preparation errors are reported with logs and prevent an unverified approval.

After review, one short question lists proposed changes not mechanically implied by repository
evidence or the owner's instructions, with a key, recommendation and reason in one or two lines
each; every risk the owner must accept in one or two lines; one line confirming everything else
follows that evidence and those instructions with `devkit proposal show <branch>` for details;
and how to answer with `approve=<version>` or `answers` naming the key to change.
The version appears only in that answer line. Full changes, triage, summary, owner_points, PR text,
version, commit and verification remain in the details. `approve=<version>` requires the recorded approval and clean
reviewed HEAD. Shared proposal publishing pushes and opens or updates the PR before triage writes:
labels, edited bodies, new topics, sub-issues and blocked-by links. Progress in
`issue-creation/<version>.json` resumes missing operations; new topics use the shared hidden
draft marker to recover lost creation responses. Excluded Issues are left untouched. External
writes happen only after approval. `devkit status` relays the approval or published PR next step;
the owner merges the PR. Do not run concurrent approvals.

Run `refine` on a `refine/<name>` branch with an open Issue carrying the topic label as `topic=<number>`, or leave it empty for a new request. It reads the full decision log, owner words, supplement, confirmed memo and answers against fixed criteria. One phase has at most three machine-check/judge rounds. Refine proposes decision records for people, binding edits about how agents work to standards, agent instructions, skills or checks, and unit/topic Issue drafts. Each unit fits one PR and includes its goal and observable completion conditions; `blocked_by` refers to other Issue draft keys in the same proposal. The machine writes proposed decisions and commits them with binding edits before fresh read-only judgment. Revisions create new proposal commits, retaining decision IDs for kept keys and deleting withdrawn unapproved records.

Group open questions into one topic when they hold the same units at the same time and cannot be decided apart: stack, hosting and the monthly cost cap can hold the foundation unit together, with the cap shaping hosting. Questions holding different units or independently decidable questions go into separate topics. Questions holding no unit go into separate, non-blocking topics for when work needs them; closely related ones may share a topic, such as contract signers and account administrators under "contracts and operating arrangements". Group by this rule to keep Issue counts manageable, rather than one Issue per question. Each topic draft names the unit keys it holds or explicitly says it holds none; waiting units link to it through `blocked_by`. Unit drafts, including retro's, contain no open questions: their work content is only the goal, observable completion conditions and product properties to preserve. Judges block mixed unit needs, unnecessary questions in a unit's blocking topic, open questions in units and mismatched blocked-by links.

In discuss, choose the topic holding the unit the owner wants to start and ask only what that topic's waiting units need. Listed points are candidates, not a checklist. Leave points holding no unit open in the memo for refine to put into a non-blocking topic.

Binding changes concern process, tools, conventions or verification, such as running configured checks before proposing a commit. Product data shapes, invariants and feature rules belong in the implementing unit Issue's testable `completion_conditions`, such as rejecting overlapping shifts for one person. Put a shared property in the first unit that builds it, and repeat the condition to preserve in later units that touch it. Code and tests then carry the product specification. The judge blocks product rules in binding changes and missing product completion conditions; this also applies to retro.

The draft requires `pr_title` (one line, at most 100 characters) and `pr_summary` (a few sentences explaining what merging puts on the base and why), written for the PR reviewer in the configured `language`. The judge reviews their meaning and wording; machine checks validate required, nonblank text and the title limits. Both fields stay unchanged in the full proposal details. Restart with `approve=<version>` only after the owner approves that version. Refine checks the recorded question, started run, owner answer and clean HEAD, marks decisions agreed and predecessors superseded in a commit, pushes the branch and opens or updates a PR with the approved title and summary, then creates Issues. It creates a parent topic from the request if absent, attaches units as sub-issues and creates new topics for remaining questions. Blocked-by links express unit order and topics waiting for other topics. It comments on the parent with decision IDs and created Issues, closing it only when no new topics remain, then updates the PR's Issue list. Results include `decision_ids`, `issue_numbers` (including the parent), `pr_number` and `pr_url`. The owner merges the PR; units wait for that merge and their blocked-by dependencies.

Refine's approval question contains only four parts:

1. Every choice or rule the writer added or derived beyond the confirmed memo and owner's words, including provisional decisions, derived agent rules and defaults: a key, recommendation and reason in one or two lines each.
2. Every risk the owner must accept, keyed and summarised in one or two lines each.
3. One line saying everything else follows the confirmed memo and owner's words, with `devkit proposal show <branch>` for details.
4. How to answer: `approve=<version>`, or `answers` naming the key to change. The version appears only here.

No version history, provenance inventory, machine fields, JSON, full bodies or exhaustive binding/Issue lists belong in the question. Facts needing no decision, including cost estimates, stay in the details. Retro and optimize use the same four parts; without a discussion memo, their first part covers every proposed change not mechanically implied by backlog/repository evidence or owner instructions. Judges block missing items or risks; repetition of settled content or other unnecessary information receives a nonblocking finding asking to cut it. The runs, discuss and optimize skills relay the question unchanged and run `devkit proposal show <branch>` when the owner asks for details.

Each cleared proposal saves its full content under branch records in `proposals/<version>.details.json`: decision bodies, binding paths and their decisions, Issue drafts with parent and blocked-by, PR title/summary, version and commit, and any verification reports. `devkit proposal show <branch>` prints the latest proposal's details file, finding state the same way as `devkit status` (including `STATE_DIR`). The approval version checks are unchanged.

The PR body starts with the approved `pr_summary` unchanged. Machine-built lists show binding paths with decision IDs, titles and reasons; linked decision records identified as a log for people; and created Issues with titles and blocked-by links. Empty lists are omitted. Headings and fixed explanations follow `language` for Japanese (`ja` and regional variants) or English, falling back to English for other languages without translating authored text. Refine uses `What to review`, `Decision records` and `Issues created`, with translated headings for Japanese. File links use the repository URL from `gh repo view --json url --jq .url` initially, or the recorded PR URL on update, with encoded branch names and paths. Approval instructions, proposal versions and internal failure/change history are not copied into the body; neither the general proposal summary nor approval text substitutes for the PR fields.

Refine and retro save approval progress in `issue-creation/<version>.json` under branch records. Retrying the same approved version resumes missing creation and links. A stable hidden body marker identifies the commit, version and draft key (with a separate parent identity); even after a lost creation response or local save, a retry searches all open and closed Issues with the configured label and adopts a match. Closed matches stay closed. Existing sub-issue and blocked-by links are accepted. Failed lookup stops creation; creation is not retried automatically. This does not coordinate concurrent creation, deduplicate parent comments or recover old unmarked Issues. Normal restart limits still apply.

Both workflows reject approval of old proposals without PR title/summary, or with an overlong or multiline title, at approval start and execution before any agreement commit, push or PR/Issue write. Make a new proposal for approval. Existing approval and commit checks and Issue progress remain in force; retries update the existing PR title and body from the currently approved content.

Use `devkit issues ready [--json]` to find ready units and topics: an Issue is open and every blocked-by Issue is closed, including unlabelled blockers and later API pages. Its parent need not be closed. JSON has `units` and `topics` arrays with `number`, `title`, `url` and nullable `parent`; text lists units first. Read an Issue and its parent, sub-issues and dependency links with `devkit issues show <number> [--json]`. Start each ready unit on its own branch:

```sh
archon workflow run requirements --branch feature/filter --detach \
  --input issue=42
```

Requirements start and prepare require an existing open Issue with the unit label and no open blockers; a refusal names the Issue and any blocking Issue. Prepare supplies the Issue and parent topic titles and bodies as request data, stripping only trailing devkit markers and their separator. Direct settled requests use `owner_words`; session findings stay in `supplement`. Every delivery request goes through `plan`.

Requirements prepare records a fresh input ID, the run ID and the number of requests read; round preserves these with its draft and judgment. Finish rejects a round after another prepare, a run change or an added request, leaving records and artifacts unchanged. Run prepare and round again before finishing; older artifacts without binding metadata also need rebuilding.

The delivery owner questions belong in requirements and plan. Implementation, code judgment and fixes return `back` to plan for newly discovered owner decisions. PR asks only about configured `ask` changes not already covered by an owner-decided row's `touches`. `pr` judges the body, commits review backlog, pushes and opens or updates a GitHub PR; the owner decides merging. For Issue requests, the body ends with `Closes #<number>` so merging closes the unit. `sync` rebases and returns `needs_judge` after conflict resolution.

The PR report judgment binds every byte of the title and body with SHA-256 material identifiers, separately from code change identity. `pr assemble` invalidates the prior pass before rewriting and preserves the round count, even when the content is identical. `pr open` rejects missing or changed content, a missing pass state or legacy judgments without these identifiers before committing backlog, pushing or invoking gh. It publishes the validated title and body snapshot, passing the body through standard input to `gh pr create/edit --body-file -`.

Small changes (a typo, a clear small bug, a test or configuration fix) do not go through the workflows: the session edits directly, following the standards, and runs `devkit checks`, which works outside a run too. The workflows are for owner decisions, changes of structure, data, contracts, security or cost, and unsettled topics. `devkit status` shows, for every branch with a recorded run, its Issue number and URL from the request record, the last outcome, the unanswered questions and the next instruction; a session uses it to pick up open requests. A question ends its run. Relay its exact ID and text; restart the named `next_workflows` workflow with the owner's verbatim `answers` JSON. Follow `back.to` for a step back. A stop or rejected start is not a pass. Restart and fix limits default to 3; only the owner's words in `continue` authorize a start that reaches the restart limit. The installed `devkit-runs` skill gives the handoff procedure.

Each branch in `devkit status --json` includes an `errors` array of strings alongside the existing fields. An unreadable answer adds its question ID, answer file path and failure reason. A configuration read or validation failure adds the configuration file name, failure reason and a statement that the restart limit is unknown to each branch; `restart_limit` is `-1` to represent an unknown limit. The array is empty when neither failed. Text output shows these failures under the branch as `unreadable: ...`. An unreadable answer is not listed as an ordinary unanswered question. The `next` instruction asks you to resolve unreadable answers and check status again. When the configuration is unreadable, it asks you to resolve the configuration first and check status again. When configuration loading succeeds, `restart_limit` reports the effective value of at least 1. Zero, negative and wrong-typed limits are configuration validation errors reported in `errors`. Status does not change records or configuration.

When the last result is a passing `judge` with `inputs.requirements_version: null`, `next` says the review is complete and asks you to relay the report to the owner. A passing judge with requirements directs you to start `pr`.

Issue lists and paginated responses, and Archon's `projects`, must be arrays; empty arrays are valid, but null, non-arrays and malformed pages are errors. Status skips a deleted project path with a reason; other path-resolution failures are reported, and invalid branch-name encoding fails with its path and cause. `doctor`, `status` and `eval judge` return stdout write errors separately from diagnostic or scoring outcomes.

The installed watcher delegates to `devkit watch <run-id> [<run-id> ...]`. It prints changes, checks unfinished runs every 30 seconds and exits when all runs end. A failed `archon workflow get` or missing Status field ends monitoring with a nonzero status and the run ID and reason, retaining stderr. Partial Status output from a failed command is discarded; monitoring does not retry retrieval failures automatically.

## After the pull request

Read comments and checks with `gh pr view <branch> --comments` and `gh pr checks <branch>`.
Pass in-scope corrections or CI failures verbatim, including their source, to `fix` as `review`
(for example `Review comment by <login> on path:line: ...` or `CI job <name> failed: <log excerpt>`).
Review fixes require passed requirements and plan and the same judged change as HEAD; the PR's
backlog commit is accepted. A passing fix needs `judge` again, then `pr` updates the PR.
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
independent judgment, it runs those cases against a temporary copy of the committed proposal's
standards and concern map, checking the clean proposal head and that the draft and approval question
have not changed since judgment. The proposal details report found/missed expectations and
provider failures in the configured language; misses and failures do not block approval. Only escape cases incur these
provider calls. Cases, corrections and consumed escape deletions belong to the proposal commit.
Existing cases remain protected; retaining, revising or withdrawing cases from a previous proposal
uses the next proposal commit. Results remain in run artifacts and the proposal record.

Comments that change or add to the request go to `requirements` on the same branch as `owner_words`,
then through the usual flow. Fix returns out-of-scope or disputed review items as `outside_requirements`
with reasons and `back` to requirements. Never answer or resolve comments on the reviewer's behalf.
For CI failures, suggest `devkit-optimize` to add the job as a configured pre-PR check.

For a branch whose last result is a passing PR, `devkit status` uses `gh` when available to show its
state, URL, review decision and failing checks. The lookup has a 15-second timeout; failures are shown
without failing status. `next` routes failing checks or requested changes to review fix/requirements,
merged PRs to `archon complete <branch>` followed by `devkit issues ready`, and closed PRs to “closed without merging”.
After merging, `archon complete <branch>` removes the worktree and local branch and keeps the remote.
Periodically run `archon isolation cleanup --merged`.

`devkit checks` compares only failing checks with the base branch's merge-base commit in a temporary detached worktree. It uses the same commands, timeouts and exit codes, after running the repository's `.archon/config.yaml` `worktree.postCreate` commands in order (each uses `postCreateTimeout` in milliseconds, default 300000). Preparation failure makes the comparison unavailable; the worktree is removed afterward. Passing checks and environment failures (75 after its existing retry, or timeouts) are not compared. `--no-base` skips comparison; skeleton checks use it. In implement and fix (`--base-on-repeat`) a check is compared only when it also failed in the previous pass, so a first failure, the usual case while writing code, costs nothing extra; judge and sync compare every failing check. A check not compared records `base: not_compared`.

Each failing result records `base: failed|passed|unavailable` and a `base_log` when run; the gate lists `preexisting` and `regressed` checks. If every failure is preexisting, implement/fix stop immediately without consuming a pass or invoking step-back. Judge reports a preexisting check as failing on the base too and stops instead of directing the branch to fix. Tell the owner which checks fail and their log paths: fix the base branch first, then restart the same workflow. That restart is `retry_after_base`, exempt from `limits.restarts`; `devkit status` says so. Retrying before the base is fixed is pointless. Regressed or unavailable comparisons retain the existing implement/fix failure behavior.

## Why the parts fit together

`<standards>/concerns.json` maps eleven qualities—structure, data, contract, security, ui, cost, test, delivery, scope, readability and document—to their principles and phase criteria. Every numbered principle has exactly one owner, counting `every_quality` as one shared assignment. Every review document except `review/README.md` must be mapped. `devkit concerns check` validates this and prepare stops on violations. The machine derives writer/judge materials from this map. Common judgment rules live in `review/README.md`; owner authority lives in `authority.md`.

Workflow criteria are pinned to the base commit recorded at request start. The shared `internal/criteria` copier selects `standards` from that commit's `.devkit/config.json`, defaulting to `.devkit/standards` if the file is absent. Requirements, plan, implement, judge, fix, refine, retro, pr and sync use its returned source paths for concern maps and materials; reused copies retain `base` and `paths` in `criteria/source.json`. Moving the current standards folder does not change those materials. Current language settings, check selection and refine/retro edit destinations still apply. `devkit eval judge` has no request-pinned criteria commit and uses the standards selection described below; retro verification evaluates the proposed standards.

`devkit judge launch` records the selected qualities in `judge/launched.json` from the same decision as its `launch_<quality>` flags; `devkit eval judge` uses the same launch processing. Code quality judges read this file and the fixed `concerns.json`: they report failures of their own principles and may report another quality's failure only when its owner was not launched. They receive no `every_quality` principles; these remain in requirements, plan, implement and fix materials. The machine checks `decision_paths` changes absent from plan rows, and scope checks authorization under `working.md#6`. A consequence that fails their own quality remains reportable, such as a missing test or a changed caller contract, even when another judge reports the defect itself.

Each quality has one `review/<quality>.md` with Decisions, Code, Blocking and Not reported sections. Plan judges choices and the skeleton; code judges bodies and conformance to passed choices. Agreed choices are carried by reference rather than judged again.

Decision records `<decisions>/D-<id>.md` are a log for people, devkit-discuss and refine. IDs derive from branch and draft key using six lowercase hexadecimal characters, extending on collision. Agreed records are replaced through `supersedes`; decision commands list, check and traverse only this log. Requirements, plan, implement, judge, fix, PR and sync neither receive nor cite it and do not check superseded records. Agent work rules belong in owner-reviewed standards, instructions, skills or checks; product properties go through unit Issue completion conditions into code and tests. Plan choices stay in delivery records; passing plan does not generate decision files. Provisional choices remain in the plan and PR body.

Document deliverables use the normal delivery flow. The code-phase `document` quality owns `writing.md` principles 2–6, 8 and 9 through `review/document.md`; principle 1 belongs to PR-body judgment and 7 to refine's approval proposal. It judges reader context, standalone explanations, actionable instructions, structure, ownership, current versus historical content, diagrams and disclosure. It has no plan-phase judge. Its routing threshold 0.6 has not been measured; existing qualities retain their measured thresholds.

Plan settles structure before detail. New structure rows explain kind, reasons for change, volatility and callers with evidence or `unknown`. After structure passes, detail may change signatures but cannot add, remove or move files or change structure rows; actual changes stop the run. Detail's `uncertain` is reserved for necessary changes to the judged structure, naming the completion condition and evidence with a verification location. General doubts and details fixable within that structure do not belong there. Non-empty entries end detail without passing and reach the owner verbatim with the existing structure question; the next plan starts from structure. Code judgment receives uncovered changed files, folder spread and ancestor siblings. Structural defects on the change's path block even if pre-existing; defects elsewhere are deferred. Jev selects qualities from each file's diff, using each quality's `routing` description and `routing_threshold` in `concerns.json`; when unavailable, `devkit judge route` runs the configured provider CLI read-only with schema validation. Configured globs and plan rows also select judges. Scope always runs; source/test changes also select test and readability. Structure routing covers function responsibilities, repeated checks, bypassed guarantees, caller state or call order, behaviour flags and values passed through layers as well as module placement. Data routing covers valid states and values and domain rules expressed as branches as well as stored data. Readability owns names, comments, conventions and shared rule definitions under `engineering.md#8`. For every added function, conversion, check or rule, it searches the whole judged tree named in `judge/target.json` for existing behaviour that must change together. Re-implementing such behaviour blocks; similar text with independent reasons for change or contracts does not. Placement and dependencies remain structure's responsibility.

Non-empty implementation `uncertain` entries reach the existing step-back judge verbatim before committing, regardless of check success. An `implement` decision continues under the usual check gate; `requirements` or `plan` returns the diagnosis without committing the unit; `environment` stops. An empty array adds no AI judgment.

`plan check` commits the skeleton before running `skeleton_check`. Checks must leave HEAD, the working tree and index unchanged, including untracked files; otherwise the round is invalid and produces no quality judgment materials. Run the check and judgment again against the updated snapshot. Before saving a pass, `plan finish` verifies the same checkout identity and rejects changes before saving plan, judgment or result history.

Before pushing, PR commits `.devkit/backlog/<encoded-branch>.json`: deferred and carried findings, mechanical-check candidates and accepted objections, with their source judgments. Only this branch's schema-valid backlog file is excluded from judged-change identity and decision detection. Identity otherwise preserves file names and every added/removed line, including whitespace, while ignoring hunk positions. No done markers are generated or excluded.

`retro`, on a `retro/<name>` branch from the base, proposes unit Issue drafts, check additions and standard edits from the backlog. Standards and check settings change only agent work; product properties go into testable unit completion conditions. A draft that passes machine checks commits real proposed files and consumed backlog deletions, recording its version and commit without file snapshots. Issue drafts stay in the proposal record. Fresh read-only judgment inspects the commit. Retro requires and reviews the same `pr_title` and `pr_summary` as refine, saving them with the version, commit and evaluation results in the details. After at most three rounds it returns the short four-part approval question described above. `approve=<version>` requires the clean proposal HEAD, pushes and opens or updates the PR, creates unit Issues without a parent topic and connects blocked-by links, then updates the PR's Issue list and records pass, `issue_numbers`, `pr_number` and `pr_url` without another file edit or commit. Its body starts with the approved summary and lists standard paths, reasons and evidence; added checks and reasons; escape sources, stages, qualities, reasons and evidence; linked case names and source escapes; and created units with blocked-by links. Escape and case headings are `Escapes` and `Eval cases`, translated for Japanese; the same language and link rules apply. Revisions and withdrawals use subsequent commits; the owner merges the correction PR before its units can start, subject to their dependencies. Old snapshot-format pending proposals are refused with instructions to restart on a new retro branch.

Records are validated against their kind's embedded schema on read as well as write. Null records, missing keys, wrong types and gaps in question IDs identify the file and cause. Initial absence, unanswered questions and the existing optional or zero `request.issue` field remain valid.

Plan completion requires every round's `settled.json` with matching round, run, phase and judgments. Refine and retro record a failure before any proposal commit with explicit `target.proposal_version: null`, `verdict: not_assessed` and the reason in observations. Committed proposal versions must be positive integers; missing or zero versions are never filled from a later proposal. An absent or successfully enumerated empty backlog is valid; enumeration failures and malformed refine/retro draft JSON stop with the path and cause.

Judge targets with `plan_version` require the matching `judge/plan.json` through changes, launch, completion and finish; missing or mismatched files require prepare again. Reviews that began without a plan still allow its absence. A named checks result must be readable before judgments or results are saved; a readable empty JSONL means zero checks. Failure to save `judge/route-jev.json` stops routing with its path and cause, even when Jev is unavailable; the Jev → provider CLI → all-quality fallback otherwise remains available.

An existing `objections.json` must be an object with an `objections` array containing string `finding` and `evidence` fields in each entry. Invalid content leaves the file and round state unchanged; an absent file or empty array means no objections. Implement and fix require the writer to rewrite missing, empty, multiline or wrong-prefix `CODE.commit_message`; implementation also rejects unit or criterion IDs in the subject. The machine adds provenance trailers without composing replacement content.

Checks treat only an absent round index with no numbered history as an initial run. Empty or invalid indexes, missing indexed results and existing next-round results require history repair; history is not overwritten to restart it. Git ref/path absence, unrelated history and an unset origin must be verified normal negative results. Git execution, object and configuration failures preserve their diagnostic. Sync does not silently repeat a failed `rebase --continue` or create an empty conflict; an unreadable `REBASE_HEAD` during conflict resolution is an error.

## Repository settings

Edit `.devkit/config.json`; `devkit config` prints effective settings. The authoritative defaults and validation are in `internal/config`, and installed values are in `kit/.devkit/config.json`.

- `provider`: `claude` (default) or `codex`; selects installed workflow AI and the Jev routing fallback. Run init again to render changed workflows.
- `sessions`: distinct session tools (`claude`, `codex`); absent defaults to both `claude` and `codex`; `[]` installs no session skills.
- `language`: owner-facing prose; built-in default `en`. Fresh init selects it from `--language` or the environment locale.
- `skills`: kit identifiers `devkit-runs`, `devkit-discuss` and `devkit-optimize` mapped to distinct installed names; defaults match those identifiers. Entries merge individually with defaults.
- `standards`, `decisions`, `eval`: repository-relative folders; default `.devkit/standards`, `.devkit/decisions`, `.devkit/eval`.
- `issues.labels.unit`, `issues.labels.topic`: non-empty GitHub Issue labels, default `devkit:unit` and `devkit:topic`. Omitted labels retain defaults; labels must already exist.
- `checks`: named shell commands with `command`, `timeout_seconds` and optional `paths`. No checks are installed by default; add the repository's real checks. Omitted paths run every time; an empty paths array selects none. Exit 0 passes, 75 retries once as an environment error, 77 means not run, and other exits fail. Timeouts are environment errors.
- `skeleton_check`: names of checks to run on the skeleton; empty by default.
- `qualities`: path globs for the fixed qualities. Document defaults are `**/*.md`, `docs/**`, `!.devkit/**` and `!.archon/**`; matching `!` exclusions take precedence over positive patterns, regardless of order. Routing and plan rows also select judges; the concern map controls their plan/code phases.
- `decision_paths`: `{paths, quality, ask}` detection rules. Defaults include dependencies, migrations, infrastructure, CI, environment examples, workflows, standards and `.devkit/config.json`. Before/after check configurations are combined so deleting a check does not bypass it.
- `limits.restarts`, `limits.fixes`: default 3 each. All three limits must be integers of at least 1; only omitted values default to 3. Invalid or unreadable configuration stops consumers rather than supplying another limit. `limits.passes`: passes `implement` gives each release step, default 3; a step that uses them up stops the run with the step and its failing checks, independent of how many steps the plan has.

Retired `docs`, `units`, `topics` and `refine.relevance_threshold` settings are accepted but ignored and omitted from effective values. Old unit/topic files are not migrated or deleted automatically; legacy request `unit` and plan-row `ref` fields are not converted. Start new Issue requests from requirements.

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

`scripts/test-kit.sh [workflow ...]` builds both executables, initializes fresh Claude and Codex repositories under `.local/tmp/`, requires workflow validation without warnings, checks principle ownership, tests fresh installations, then copies source-only executed fixtures and tests them for both providers and runs `go test ./...`. Its isolated Archon home uses `$HOME/.archon/plugins`, overridable with `KIT_ARCHON_PLUGINS`. Every workflow has an executed main path: bash runs for real and AI is stubbed. PR publication is stubbed; sync exercises a real local rebase conflict. `cmd/devkit-fixture` supplies test-only setup and file writes and is absent from the shipped binary. The plumbing test rejects YAML/prompt calls to unregistered commands. Go tests cover the evaluation with fake CLIs. Fake gh and local bare remotes verify refine/retro PR creation and updates, approved title/summary use, change/decision/Issue/dependency and retro trace/case lists, Japanese/English fixed text, the 100-character title boundary and rejection of old proposals before external writes.

See the [workflow guide](kit/.archon/workflows/dev/README.md) for artifacts and execution contracts, and [design rationale](docs/design.md) in Japanese. Update these explanations alongside their authoritative code, YAML, schemas and standards.

## Current limits

There are no dependency facts from analysis tools; current materials use Git facts. Source AI nodes use Claude; installed workflows and the read-only routing fallback use the configured provider. Codex workflow judges cannot be prevented from writing; checkout mutations are detected by Archon. If Jev and the CLI are both unavailable, routing selects every routable quality. Pull request publication supports GitHub through `gh`. Live AI behavior and external PR publication are outside fixture coverage.
