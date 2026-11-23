---
name: devkit-runs
description: Start and monitor Archon development workflows, route requests through discussion and refinement or downstream delivery, and relay recorded questions, answers and approvals to the owner.
---

# Run development workflows

Use the installed Archon CLI skill, when available, for CLI syntax; otherwise consult `archon --help`
and `archon workflow --help`. This skill defines the development sequence and owner handoffs.

## Before starting

Run `devkit doctor` for missing requirements and part-specific tools. Read `provider` and `sessions`
from `devkit config`: workflow AI and the Jev fallback use the provider; this session uses the skills
installed for its own tool under `.claude/skills/`.

When `devkit config` shows no `checks`, the kit has not been fitted to this repository yet: say so
and offer the `devkit-optimize` skill before starting development workflows.

Run from the repository root. Invoke `archon` directly, without a project-specific command wrapper.
`ARCHON_HOME` is optional: retain an existing project setting or export the same chosen value in every
shell used to start, inspect and watch runs. Without it, Archon uses its normal default. Do not invent a
project path. A run copies the checkout's `.archon/` when it starts; do not start while those files are
being edited. For an Issue request, check its recorded blocking dependencies before beginning and do
not start a blocked Issue. Preserve any project-specific concurrency constraints rather than inventing
a limit for every repository.

## Choose the sequence

Use a branch for each topic or work unit, and start workflows with `--branch <name> --detach`.

- Unsettled request: `devkit-discuss` → `refine` one topic at a time → exact-version approval →
  publish and merge the refine pull request → for each settled work unit, on its own branch:
  `requirements` (`unit=<id>`) → `plan` → `implement` → `judge` ⇄ `fix` → `pr`.
- A settled development request: `requirements` → `plan` → `implement` → `judge` → `pr`.
  Every request goes through `plan`, including a trivial request with few or no rows.
- Review only, without requirements: `judge` produces a report only. If the change needs an owner
  decision or a plan, start `requirements` describing the change, then follow the full delivery flow.
- Fix findings on a branch with passed requirements and plan: `fix` → `judge` → `pr`. Repeat `fix` and `judge` while blocking findings remain,
  subject to the configured fix and restart limits.
- Update against the base branch: `sync` → `judge` when `needs_judge` is true → `pr`.
- Correct the process after backlog reaches the base branch: `retro` → exact-version approval →
  publish and merge the correction pull request → eligible repair units through the same unit flow.

Workflows do not invoke one another; the session chooses the next one from the recorded result.

For refine, use a `refine/<name>` branch. One run settles one topic in decide and document phases,
each with a three-round limit. Use `devkit decisions topics` to find open topics with agreed decision
dependencies and pass the selected ID as `topic`; leave it empty for the initial request topic. Pass the owner's words verbatim as `owner_words`, your
research separately as `supplement`, and the owner-confirmed discussion memo verbatim as `memo`.
Do not add unsaid requirements to owner words. An Issue's body is owner words; related Issue/PR bodies,
comments and researched facts belong in supplement so the read-only judges have the materials.

Start a unit only after the refine pull request that created it has merged.
Run `devkit decisions units` (or `--json`) in the base checkout, updated from the remote, to find open units marked ready,
and start ready units. Read the selected unit's complete file from the configured units folder.
PR records its done marker; the unit becomes done on the base branch when its pull request merges. Make a
separate branch for the unit and pass its ID as requirements' `unit`, for example `--input unit=U-a1b2c3`.
Prepare loads the unit and its agreed decisions as constraints. Use `owner_words` for direct requests.
Keep the branch and PR for later corrections unless the owner asks for another. After refine passes,
publish its branch through the repository's PR process; refine itself only commits. Continue with the
next pickable topic, using `devkit-discuss` when choices need discussion. Use `devkit decisions units`
again after merging a unit PR to find units that are now ready.

## Find and relay results

Read where every request stands with:

```sh
devkit status                # every branch with a recorded run, newest first
devkit status <branch>       # one branch
devkit status --json         # the same, for reading fields
```

For each branch it shows its unit ID from the request record, the last workflow and its outcome,
the questions still unanswered (ID and full text), the unresolved items of a stop, the premise and observation of a `back`, and `next`: the
workflow to start and the input it needs. Use it after a watched run ends, at the start of a session
to pick up requests left open, and whenever the owner asks where things are. A run with
`no result` is still running or failed: check it with `archon workflow get <run-id>`. A run whose
start conditions did not hold records nothing, so `devkit status` still shows the previous state: when a
run you started is missing there, read why it did not start with `archon workflow get <run-id>`. Do not read or
pass the record folder to AI nodes; when `devkit status` cannot find the records (it names why), say
so instead of guessing a path.

A `question` result ends that run. Relay every question and its candidates verbatim to the owner,
including its ID. Relay a choice recommendation and reason only when supplied, and never add one to a
question about facts. Relay the owner's answers verbatim, without answering omitted questions for them.
Start the question's `next_workflows` workflow on the same branch with:

```sh
archon workflow run <workflow> --branch <branch> --detach \
  --input 'answers=[{"question_id":"Q1","answer":"<owner answer verbatim>"}]'
```

Use shell-safe quoting or the CLI's supported file input when text contains quotes or substitutions.
Do not change the answer to fit a shell example.

For refine's or retro's single proposal approval question, show the full question and the exact version. Only
when the owner approves that version, start that workflow again on the same branch with
`--input approve=<version>`. To request changes, restart it with the owner's words in `answers` for the
approval question's ID. Never synthesize approval or use `approve` for an open-point question.

Run retro on a `retro/<name>` branch created from the base. It reads that branch's backlog, proposes
units, check additions and standard edits, then asks one approval question after machine and fresh
read-only judgment. Approval applies corrections and deletes consumed backlog files in one commit.
An empty backlog passes without an approval question. Publish and merge the correction PR through
the repository's PR process before starting its units.

Owner questions about the delivery belong in refine, requirements and plan. PR asks only about
configured `ask` changes not covered by an owner-decided row's `touches`; do not ask again for covered
choices. Retro asks approval of its corrections. Implement, judge and fix route owner choices back
to plan. Use the standards' authority document for what the owner decides.

`implement`, `judge` and `fix` return `back` to `plan` when they discover an owner decision on a delivery branch.
A judge without requirements never returns `back`: a needed decision or plan is a `stop` with
"this change needs requirements and a plan: start `requirements` describing it".
A review-only pass finishes the report; it does not authorize a fix-to-PR shortcut.
Start `plan` on the same branch; it receives pending owner rows and asks the owner there. A plan
`stop` caused by a required structure change needs a new plan run. A passing fix needs judge again
before PR; a passing implementation proceeds to judge. A passing judgment with passed requirements and plan proceeds to PR unless
blocking findings require fix. PR returns its URL for the owner's merge decision.
For other `back` results, start the workflow named by `back.to`. Its prepare loads every valid,
unconsumed premise and observation into `returned`; the writer addresses those first.

For `stop`, explain the state and unresolved items. Restarts are counted per branch; the configured
`limits.restarts` default is 3. On reaching it, relay that fact and obtain the owner's words allowing
continuation. Pass those words verbatim as `--input continue=<owner words>`. Do not supply `continue`
before the start reaches its limit. Fix attempts also obey configured `limits.fixes`. A `not_started`
result is not a pass: inspect the `start` output and relay what prevented starting.

## Watch runs

After starting, attach the session's background monitor, when available, to:

```sh
bash .claude/skills/devkit-runs/scripts/watch.sh <run-id> [<run-id> ...]
```

It prints a line when statuses change and exits when all runs end. Do not filter
`archon workflow attention --watch` through grep; that can hide requests for a decision. Do not poll
repeatedly alongside the watcher. If the monitor times out (some sessions stop it after 30 minutes),
reattach it to runs that have not ended.

For an unexpected `paused` run, inspect `archon workflow get <run-id>` and the referenced materials,
then relay its question verbatim. Never answer on the owner's behalf; use the answer command that run
specifies after the owner replies. Check that monitoring continues. Development workflows normally
finish with recorded questions rather than waiting inside Archon.

For `failed`, read `workflow get` and the end of the logs, and explain which node failed and why. Fix a
workflow defect in the main checkout through the project's development process before rerunning the
same workflow on the same branch. If the request changes during a downstream run, restart from
`requirements` with the changed request. After `pr`, give the owner the PR URL and relay unverified
items and carried findings. Do not merge the PR for them.
