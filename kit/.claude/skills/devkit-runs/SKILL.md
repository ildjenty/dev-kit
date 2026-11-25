---
name: devkit-runs
description: Start and monitor Archon development workflows, route requests, relay owner handoffs, and record defects found after merge, including those repaired directly outside workflows.
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

## Decide whether the request needs the workflows

The workflows exist for work that needs what they provide: owner decisions recorded and asked at the
right time, a plan judged before code, independent judgment of the result. A small change gains
nothing from them and costs many AI runs. Never send a small request through the full flow.

Use the workflows only when the request needs one of these:

- a decision the owner makes under the standards' `authority.md` (a choice that is hard to change
  later, cost, permissions or exposure, a new external service, a change of the standards or checks);
- a change of structure, stored data, a public contract, security or running cost that is worth a
  plan: new modules or boundaries, a schema or migration, an interface others call, authorization;
- a topic that is not settled yet (start with `devkit-discuss` and `refine`).

Everything else is done directly in this session, without starting a workflow: a typo or wording,
a small bug whose cause is clear, a test fix, a configuration tweak within agreed limits, a dependency
update the repository already allows. Follow the standards in the configured standards folder while
editing, then run the configured checks on the change with `devkit checks` (it selects the checks
that apply to the changed files and prints the result; logs are in the folder it names). When the
owner wants an independent review of such a change, commit it on a branch and run `judge` alone: on a
branch without requirements it returns a report and nothing else.

When you cannot tell which kind the request is, ask the owner once, naming the two ways and why it
is unclear. When the owner explicitly asks for the workflows, use them.

## Record defects found after merge

When the owner reports a defect in merged work, record the escape however it will be fixed,
including a small direct fix outside the workflows. Write the owner's description verbatim to a
file, find the entering commit or PR with `git log`/`git blame` (ask the owner if it is unclear),
then run `devkit backlog escape --defect-file <file> --entered <ref>`; optionally add `--fixed <ref>`.
Commit the printed record with the fix or through the repository's PR process so it reaches the
base branch. Retro traces the missed stage and adds and evaluates a judge case when applicable.

## Choose the sequence

Use a branch for each topic or work unit, and start workflows with `--branch <name> --detach`.

- Unsettled request: `devkit-discuss` → `refine` one topic at a time → exact-version approval →
  approval run publishes the refine PR → owner merges it → for each ready unit, on its own branch:
  `requirements` (`issue=<n>`) → `plan` → `implement` → `judge` ⇄ `fix` → `pr`.
- A settled request that needs the workflows: `requirements` → `plan` → `implement` → `judge` → `pr`.
  Once a request is in this flow it goes through `plan`; small requests are not put in it at all.
- Review only, without requirements: `judge` produces a report only. If the change needs an owner
  decision or a plan, start `requirements` describing the change, then follow the full delivery flow.
- Fix findings on a branch with passed requirements and plan: `fix` → `judge` → `pr`. Repeat `fix` and `judge` while blocking findings remain,
  subject to the configured fix and restart limits.
- Update against the base branch: `sync` → `judge` when `needs_judge` is true → `pr`.
- Correct the process after backlog reaches the base branch: `retro` → exact-version approval →
  approval run publishes the correction PR → owner merges it → eligible units through the same flow.

Workflows do not invoke one another; the session chooses the next one from the recorded result.

Pass owner text, answers, memos and review comments without putting them in the command line:
`--input` takes only `name=value`, and quotes, `$` or backquotes in the text would break or change
the command. Write each value to a file outside the checkout with your file-writing tool (for
`answers`, valid JSON), then pass it as `--input "<name>=$(cat <file>)"`. Text read this way is not
interpreted again by the shell. Never edit the text to make it fit a command.

For refine, use a `refine/<name>` branch. One run handles one topic in a single phase with at most
three rounds. Pass the number of an open Issue with the configured topic label as `topic`; leave
it empty for a new request. Pass the owner's words verbatim as `owner_words`, your research
separately as `supplement`, and the owner-confirmed discussion memo verbatim as `memo`.
Do not add unsaid requirements to owner words. An Issue's body is owner words; related Issue/PR bodies,
comments and researched facts belong in supplement so the read-only judges have the materials.
For example, with each text written to a file first:

```sh
archon workflow run refine --branch refine/<name> --detach \
  --input "owner_words=$(cat <owner words file>)" --input "supplement=$(cat <supplement file>)" \
  --input "memo=$(cat <memo file>)" --input topic=<issue number or empty>
```

Run `devkit issues ready` (or `--json`) to find open unit and topic issues whose blockers are all closed.
Units with an unmerged source PR appear separately as waiting, with that PR; requirements refuses them.
Read a selected issue with `devkit issues show <n>` and start a ready unit on its own branch with
requirements' `issue`, for example `--input issue=42`. Prepare supplies its title and body and the
parent topic's title and body as request data alongside owner words and supplement. Use `owner_words`
for direct requests. The PR body ends with `Closes #42`; merging the PR closes the unit issue.
Keep the branch and PR for later corrections unless the owner asks for another. Refine's approval run
publishes its branch through a machine bash node; the owner merges the PR. Continue with the
next topic, using `devkit-discuss` when choices need discussion. Run `devkit issues ready` again after
merging a unit PR to find work whose blockers are now closed.

## Find and relay results

Read where every request stands with:

```sh
devkit status                # every branch with a recorded run, newest first
devkit status <branch>       # one branch
devkit status --json         # the same, for reading fields
```

For each branch it shows its issue number and URL from the request record, the last workflow and its outcome,
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
archon workflow run <workflow> --branch <branch> --detach --input "answers=$(cat <answers file>)"
```

where the answers file holds `[{"question_id":"Q1","answer":"<owner answer verbatim>"}]`.

For refine's or retro's single proposal approval question, show the full question, including the
proposed `pr_title` and `pr_summary`, and the exact version. Only
when the owner approves that version, start that workflow again on the same branch with
`--input approve=<version>`. To request changes, restart it with the owner's words in `answers` for the
approval question's ID. Never synthesize approval or use `approve` for an open-point question.

Refine commits proposed decision records and binding edits about agent process, tools, conventions
or verification to standards, agent instructions, skills or checks before fresh read-only judgment.
Product data shapes, invariants and feature rules belong in the implementing unit Issue's testable
completion conditions. Shared properties appear in the first unit that builds them and are repeated
in later units that touch them; they do not go into binding edits. It reads the full decision log; delivery never consumes
it. On approval, refine marks records agreed, commits that change, pushes and opens or updates its PR,
then creates unit issues under
the selected topic (creating a parent topic if absent), new topic issues and blocked-by links.
It closes the parent with a comment when no new topics remain, otherwise comments without closing.
Relay `decision_ids`, `issue_numbers`, `pr_number` and `pr_url`; keep `devkit-discuss` active until the discussion is in Issues.
The machine uses the approved `pr_title` and `pr_summary` unchanged for the PR, followed by lists of
approved changes, decision records and created Issues with blocked-by links. It does not reuse
approval instructions or the general proposal summary as PR text.
The owner merges it; units become ready after that merge and after their blockers close.
When `publication` is `nothing_to_publish`, relay that no commits remain beyond the base.

Run retro on a `retro/<name>` branch created from the base. It reads that branch's backlog, proposes
unit Issue drafts, check additions and standard edits, commits the real files and consumed backlog
deletions. Standards and check settings change only how agents work; product properties use unit
completion conditions as in refine. It asks one approval question showing the version and commit after machine checks, fresh read-only
judgment and escape verification. Approval requires that version's commit to remain the clean branch
head; it pushes and opens or updates the PR, creates unit issues without a parent topic, connects
blocked-by links and records `issue_numbers`, `pr_number` and `pr_url`, without creating another commit. Requested changes create the
next proposal commit on that history; withdrawn edits are undone by a new commit. Old snapshot
proposals must be restarted on a new retro branch.
An empty backlog passes without an approval question. The approval run publishes the correction PR
in a machine bash node. The owner merges it; units become ready after that merge and their blockers close.

Both workflows reject approval of old proposals without valid PR title/summary before any agreement
commit, push or PR/Issue write; make a new proposal for approval. Retrying an approved version updates
the existing PR title and body from that approved content.

Both workflows keep Issue drafts in the proposal record and create Issues only on approval. If an
approval fails during publication or Issue creation, inspect the failure and restart the same workflow, branch and
approved version when the cause is resolved. The PR is found by head branch and reused; a push failure
stops before Issue creation. Creation progress is saved; stable hidden body markers
let a retry adopt even an Issue whose creation response or local save was lost, including closed
issues, and complete missing sub-issue and blocked-by links. Do not manually recreate the drafts or
reopen adopted issues. This does not prevent concurrent creation or duplicate parent comments after
a lost response; do not run approval concurrently. Normal restart limits still apply.

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

For a `stop` with `stop.base_failed: true`, tell the owner that the named checks already fail on the
base branch. Relay the failing checks and their log paths from `stop.unresolved`. Fix the base branch
first, then start the same workflow again; `devkit record limit` classifies this as `retry_after_base`,
not counted against `limits.restarts`, even at the limit. Retrying before the base is fixed is pointless.
Do not send this stop to fix or step-back. Implement/fix do not consume a pass for this failure;
judge records that the check fails on the base too.

For `stop`, explain the state and unresolved items. Restarts are counted per branch; the configured
`limits.restarts` default is 3. On reaching it, relay that fact and obtain the owner's words allowing
continuation. Pass those words verbatim as `--input continue=<owner words>`. Do not supply `continue`
before the start reaches its limit. Fix attempts also obey configured `limits.fixes`. A `not_started`
result is not a pass: inspect the `start` output and relay what prevented starting.
When it names a missing `devkit <area> <subcommand>`, the installed kit and binary come from different
versions. Tell the owner the command and the installing version when given, and relay the remedy:
run `devkit init` in the repository with the matching binary, or install the version that installed
these workflows. Do not retry the run.

## After the pull request

For a passing PR, `devkit status <branch>` reads the GitHub state, URL, review decision and failing
checks when `gh` is available. Lookup failures are reported without preventing status output.
Read the feedback before choosing the next workflow:

```sh
gh pr view <branch> --comments
gh pr checks <branch>
```

Read the actual review comments and failing job logs as well; these summaries alone may not contain
inline comments or the failure cause. Preserve the text verbatim with provenance, for example
`Review comment by <login> on path:line: ...` or `CI job <name> failed: <log excerpt>`.

- A comment that repairs the change within what was asked goes to `fix` on the same branch with
  the sourced text as `review` (`--input review=<text>`). A passing review fix needs `judge` again,
  then `pr` updates the existing pull request. The backlog commit does not invalidate
  the judged change for this start.
- A comment that changes or adds to what was asked goes to `requirements` on the same branch with
  the comment as `owner_words`, then the usual plan → implement → judge → pr flow. An item the fix
  writer believes is outside the requirements or plan, or wrong, returns in `outside_requirements`
  with a reason and `back.to: requirements`; start requirements there for the owner to settle it.
- A CI failure goes to `fix` with the job name and verbatim failure excerpt as `review`. Suggest
  adding that job as a configured check through `devkit-optimize` so it is caught before the PR.

Pass review and owner text through files as described above. Never answer or resolve
review comments on the reviewer's behalf. The owner decides whether to merge.

After the merge, run `archon complete <branch>`: it removes the worktree and local branch and keeps
the remote branch. Run `devkit issues ready` to find the next work. From time to time, run `archon isolation cleanup --merged` for merged worktrees.

## Watch runs

After starting, attach the session's background monitor, when available, to:

```sh
bash .claude/skills/devkit-runs/scripts/watch.sh <run-id> [<run-id> ...]
```

It delegates to `devkit watch`, prints a line when statuses change, checks unfinished runs every
30 seconds and exits when all runs end. A retrieval failure or missing Status field exits nonzero
with the run ID and reason, including stderr; it does not retry automatically or adopt partial
Status output from a failed command. Inspect and resolve that failure before reattaching. Do not filter
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
