---
name: devkit-optimize
description: Fit a freshly installed dev-kit to this repository. Use right after `devkit init` in an existing repository, or when its build, tests, conventions or folder layout changed. Sets `.devkit/config.json` and `.archon/config.yaml` from what the repository actually does (checks, paths that select judges and decisions, base branch, worktree preparation), reconciles the installed standards with the repository's own conventions, fits the routing descriptions to its layout, and points agent instructions at the kit's skills.
---

# Fit dev-kit to this repository

`devkit init` installs settings and standards that fit no repository in particular: no checks, path
globs for every ecosystem, the base branch `main`, and principles written without knowing this
repository's conventions. Until they are fitted, the workflows run no verification of their own,
launch judges for paths that do not exist here, miss the ones that do, and judge existing, agreed
conventions as findings. This skill reads the repository, proposes changes with the evidence for
each, lets the owner decide what only the owner knows, writes what the owner approved, and proves
the result works in a fresh worktree.

What you may change, and only after the owner approved that change:

- `.devkit/config.json` and `.archon/config.yaml`;
- documents in the standards folder (the `standards` setting), including `routing` in `concerns.json`;
- the repository's agent instruction files (`CLAUDE.md`, `AGENTS.md`), by adding a short section.

Never change the workflows, the repository's code, CI or scripts here: say what you would change and
why, and leave it to the owner. Everything you propose rests on something you read or ran; where you
could not observe it, say so instead of guessing.

## 1. Check that the kit itself works

Run from the repository root and stop on the first failure, reporting it:

```sh
devkit version
devkit config                 # the merged settings; fails when .devkit/config.json is invalid
devkit concerns check         # every principle has exactly one owner
archon --version
archon validate workflows
```

Run `devkit doctor` for required tools and partial capabilities: it checks each configured session
CLI and the configured provider's CLI fallback, gh authentication and origin, and key presence.
Do not print the key. Setting it sends changed files' diffs, and new decisions plus their citing
documents, to TypeSafe; that is the owner's choice, so only report the state. This session
reads skills under `.claude/skills/`. `provider` chooses workflow AI and fallback;
`sessions` chooses skill installation locations. Keep both settings unless the owner asks to change them.

## 2. Read the repository

Read, do not change. Collect each fact with where you found it.

- **Base branch**: `git symbolic-ref --short refs/remotes/origin/HEAD`, else the branch the CI
  definitions and the default remote agree on.
- **Language of the people**: the language of the README, recent commit messages, issues and existing
  documents. This becomes `language`, the language of everything the owner reads.
- **What CI runs**: every CI definition (`.github/workflows/`, `.gitlab-ci.yml`, and so on) and the
  commands in its jobs. What CI already enforces is the best source for checks: the repository has
  agreed those must pass.
- **How things are run locally**: task runners and manifests (`Makefile`, `justfile`, `Taskfile`,
  `mise.toml`, the scripts of package manifests), the toolchain versions they pin, and what a new
  clone needs before anything runs (dependency install, code generation, containers, env files).
- **Layout**: `git ls-files` grouped by top-level and second-level folder. Find where each of these
  lives, if it exists: stored-data definitions (schemas, migrations, models, SQL); public interfaces
  (HTTP or RPC handlers, routes, interface definition files, exported packages); authentication,
  authorization, input validation, secrets and permission configuration; screens (components, views,
  templates, styles, user-facing text); anything that costs money to run (infrastructure definitions,
  schedules, clients of paid APIs or models and their limits); tests; documents.
- **Conventions already agreed here**: agent instruction files (`CLAUDE.md`, `AGENTS.md`),
  contribution and style guides, linter and formatter configuration, the commit history's message
  form, review checklists, and how the existing code actually does the things the standards have
  principles about (comments, naming, error handling, tests, module layout). A convention counts when
  it is written down or followed consistently across the code, not when it appears once.

## 3. Draft the settings

Start from the current files and keep what already fits. For each change, keep the evidence.

**`.devkit/config.json`**

- `language`: as observed.
- `docs`: the folder where the repository's product and design documents live. Keep `standards`,
  `decisions`, `units` and `topics` unless a path collides with something that exists.
- `checks`: one entry per verification the repository relies on, with `name`, `command`, `paths` and
  `timeout_seconds`.
  - Take the commands CI runs. A check that CI does not run is added only when the owner asks.
  - A command runs with `sh -c` from the repository root of a fresh worktree, non-interactively. Exit
    code 0 passes; 75 means the environment failed, not the change (it is retried once); 77 means the
    check could not run and is reported as skipped; anything else fails. When a command can fail for
    environmental reasons (a container that did not start, a network fetch), propose a small wrapper
    script that returns 75 for those, and leave writing it to the owner.
  - `paths` are globs (`**`, `*`, `?`) of the files whose change makes the check necessary. Omit
    `paths` only for a check that must always run; an empty list selects nothing. Scope each check so
    a documentation change does not run the test suite, and a source change always does.
  - Run every command once on the base branch. Record whether it passed and how long it took, and set
    `timeout_seconds` to about three times that, at least 60. A check that fails on the base branch
    blocks every request: do not configure it as it is; report it and ask the owner whether to fix it
    first, scope it, or leave it out.
- `skeleton_check`: the names of the checks that can verify signatures without function bodies (a
  compile or type check), when the language has one. Otherwise leave it empty.
- `qualities.<quality>.paths`: the globs whose change means that quality's judge should look, from the
  layout you observed, for `data`, `contract`, `security`, `ui`, `cost` and `test` (source and test
  files). Remove default globs that match nothing here and add the repository's real folders. A broad
  glob launches a judge on every change; a missing one leaves the choice to routing alone.
- `decision_paths`: the paths whose change is itself a decision that the plan must record: the
  dependency manifests and lockfiles that exist here, the migration folders, infrastructure and deploy
  definitions, CI definitions, env examples, and the kit's own files (`.archon/**`, the standards
  folder, `.devkit/config.json`). Keep `ask: true` on those the owner must allow before a pull request
  (cost, permissions and exposure, CI, the kit's own rules); ask the owner about any you are unsure of.
- `limits`, `skills`, `provider` and `sessions`: leave as they are.

Confirm that every glob you keep or add matches at least one tracked file, and list those that match
nothing:

```sh
git ls-files -- ':(glob)<pattern>' | head -3
```

**`.archon/config.yaml`**

- `worktree.baseBranch`: the base branch.
- `worktree.postCreate`: the commands that make a fresh worktree able to run the checks (trust and
  install the toolchain, install dependencies, generate code), in order, and `postCreateTimeout` when
  they are slow. A worktree contains only tracked files: anything untracked that the checks need (env
  files, local databases) has to be created by these commands or supplied by the environment. When the
  repository already has an Archon configuration, add to it; do not replace it.

## 4. Reconcile the standards with the repository's conventions

The judges read the standards folder as their criteria. Where an installed principle disagrees with
a convention this repository already agreed on, every request would be judged against a rule nobody
here chose, and existing code would be reported as findings. Find those disagreements before the
first run; do not look for anything else to improve in the standards.

- For each principle and review criterion, compare it with the conventions you collected in step 2.
  List only real disagreements: the standard forbids or requires something, and the repository
  consistently does the opposite or has a written rule that says otherwise. A standard the repository
  simply has no opinion on is not a disagreement.
- For each, propose one of: **keep the standard** (the repository's practice is the thing to change,
  later, through normal requests); **adapt the standard** (rewrite the principle, its applicability or
  its exemptions so that it states what this repository agreed, keeping the principle's purpose and
  its blocking condition where the purpose still holds); or **take the repository's rule** (replace
  the wording with the repository's own, citing where it is written). Give your recommendation and why;
  the standards' own `README.md` says how principles are written.
- `git.md` (branching and commit message form) and `authority.md` (what the owner decides and what is
  delegated) are almost always repository-specific: compare them with the history and the instruction
  files and propose the wording that matches.
- A proposal never removes a principle outright and never weakens a blocking condition about security,
  data loss or what the owner decides without the owner asking for exactly that.

**Routing descriptions.** In the standards' `concerns.json`, each routed quality has a `routing`
sentence: the concrete kinds of change that touch it. Propose wording that names this repository's own
terms and folders (for example which folders hold stored data, public interfaces, screens, or paid
services), keeping it one sentence per quality. Do not change `routing_threshold`, `principles`,
`plan` or `code` here.

## 5. Point agent instructions at the kit

Sessions use the kit only when something tells them to. Propose a short section for the
repository's agent instruction files for the configured sessions (`CLAUDE.md` for Claude, `AGENTS.md`
for Codex; retain existing files), in the
language of that file, saying: development requests go through the kit's workflows; which skill
starts and watches runs and which is for discussing an unsettled topic (use the names in `skills` of
`.devkit/config.json`); and that questions recorded by a workflow are relayed to the owner, never
answered for them. Add the section; do not rewrite what is already there, and point out, without
editing, any existing instruction that contradicts the standards or the workflows.

## 6. Let the owner decide

Show everything before writing anything:

- the two settings files as diffs, each setting with its evidence (the CI job, the script, the
  folder), the result and duration of each check on the base branch, and the globs that match nothing;
- each disagreement between a standard and a convention, with both texts, where the convention is
  written or seen, your recommendation and the exact edit;
- the proposed `routing` sentences next to the current ones;
- the section for the agent instruction file.

Then ask, each with the candidates and your recommendation where there is a choice:

- checks that failed on the base branch, and checks CI runs that you propose to leave out;
- which changes need the owner's permission before a pull request (`ask`), where the repository does
  not make it obvious;
- the base branch or the language, only when the evidence disagrees;
- for each standards disagreement: keep, adapt or take the repository's rule.

Do not ask about what the repository already settles. Write only what the owner approved; a standards
edit the owner did not answer stays unwritten and is listed in the report.

## 7. Prove it in a fresh worktree

The workflows run in worktrees, so a setting that works in this checkout proves little. After writing:

```sh
devkit config && devkit concerns check && archon validate workflows
git worktree add --detach "$(mktemp -d)/devkit-optimize" HEAD
```

`devkit concerns check` must still pass after the standards edits: a principle you renumbered, removed
or added has to be owned exactly once in `concerns.json`. In the worktree, with the edited files
copied in (they are not committed yet), run the `postCreate` commands in order, then each check's
command. Record pass or fail and the time for each, then remove the worktree
(`git worktree remove --force <path>`). A failure here is what every run would hit: fix the setting,
or report what the repository lacks, and run it again.

## 8. Report and hand over

Report what you changed and why, what you ran with its result, what you could not verify, and what
you recommend but did not change (wrapper scripts for environment failures, standards disagreements
left open, instructions that contradict the workflows).

The changes take effect for runs started after they are on the base branch: the kit treats its own
settings and standards as decisions, and a run reads the files of its own checkout. Tell the owner to
commit them to the base branch (do not commit or push yourself unless asked).

Two things this skill does not do, and what to do instead:

- **Decisions the repository already made** (in design documents, decision records, or people's
  heads) are not in the kit's decision records yet, so the workflows cannot cite or protect them.
  Suggest recording them with the `refine` workflow, one topic at a time, starting from the documents
  that state them.
- **Structural defects that already exist** are reported by the structure judge when a change passes
  through them, and block that change. Tell the owner to expect this on the first requests in old
  areas; they are fixed as part of those requests or collected through the backlog and `retro`.

A cheap first trial is the `judge` workflow on an existing small branch: it exercises the checks and
the judges without changing code.
