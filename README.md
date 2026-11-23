# dev-kit

dev-kit gives Archon a portable development process: the owner decides what matters, a change stays in one place, and the process corrects itself from observed failures. Nine workflows, two session skills, repository-owned standards and the Go binary `devkit` work together. AI sessions write and judge; the binary checks facts, enforces transitions and preserves evidence.

## Install

Target repositories need Git, Archon with an authenticated Claude provider, and `devkit` on `PATH`. The signed-in `codex` CLI is needed only for the routing and document-reflection fallback when `TYPESAFE_API_KEY` is unset or Jev cannot answer. Pull requests use an authenticated GitHub CLI (`gh`) and the `origin` remote. Supply the tools required by your repository's checks. Go and mise are needed only to build and test dev-kit itself.

From this checkout:

```sh
mise install go
scripts/install.sh
export PATH="$HOME/.local/bin:$PATH"
cd /path/to/your-repository
devkit init
```

`scripts/install.sh --bin /your/bin` changes the binary destination. `devkit init [--force] [dir]` installs the embedded kit without committing: `.archon/workflows/dev/`, `.claude/skills/dev-discuss/`, `.claude/skills/dev-runs/`, initial standards in `.devkit/standards/`, and configuration. Kit-owned workflows and skills are refreshed; existing repository-owned configuration and standards are preserved unless `--force` is supplied. Review and commit the installed files before starting runs. A committed base branch is required for workflow execution.

Optionally set `TYPESAFE_API_KEY` in the workflow environment to use Jev for routing and document reflection. For `judge` routing, each changed file's diff is sent to TypeSafe's API. For `refine`, the documents that cite new decisions and those decisions' titles and bodies are sent. Without a key, nothing is sent to TypeSafe and Codex performs these checks. The key is read only from the environment and is never recorded. A failed or malformed Jev result also falls back to Codex; partial results are discarded. `TYPESAFE_API_URL` overrides the endpoint for tests.

The supplied `.archon/config.yaml` disables bundled default workflows and commands and sets `worktree.baseBranch` to `main`. Incorporate these settings into existing Archon configuration; change the base branch there when necessary. Keep workflow sources committed and unchanged while starting runs.

## Use the flow

```text
dev-discuss → refine one topic → approval → refine PR → owner merges
  → next pickable topic, or for each settled work unit:
    requirements (unit=<id>) → plan → implement → judge ⇄ fix → pr
    sync → judge when needs_judge → pr
  → retro → approval → correction PR → owner merges
```

`dev-discuss` compares the owner's words with agreed decisions and prepares a confirmed memo in the session. `dev-runs` starts and watches workflows and relays their recorded results. Workflows do not invoke one another.

Run `refine` on a `refine/<name>` branch. One run settles one topic: first decisions and next topics, then documents and units for settled areas. Each phase has its own three-round machine-check/judge loop. In the document phase, the machine requires a citation for every new decision; Jev checks reflection at probability >= 0.5, or Codex checks it when Jev is unavailable. Missing reflection blocks the round, and Claude judges wording. Open points end with `question`; a cleared proposal ends with an approval question. Restart with `approve=<version>` only for the version the owner approved. Approval marks decisions agreed and the current topic settled. Publish the branch through the repository's PR process; merge its refine PR before starting any unit it created. `refine` itself does not publish a PR.

Find open topics whose decision dependencies are agreed with `devkit decisions topics`. Pass a selected topic as refine's `topic`. For a work unit, honor its `depends_on`, make a separate branch and pass its ID directly:

```sh
archon workflow run requirements --branch feature/filter --detach \
  --input unit=U-a1b2c3
```

Prepare reads the unit and its cited agreed decisions as constraints. For a direct settled request, use `owner_words` instead; keep session findings in `supplement`. Every delivery request goes through `plan`, even when its decision list is empty.

The delivery owner questions belong in requirements and plan. Implementation, code judgment and fixes return `back` to plan for newly discovered owner decisions. PR asks only about configured `ask` changes not already covered by an owner-decided row's `touches`. `pr` judges the body, commits review backlog, pushes and opens or updates a GitHub PR; the owner decides merging. `sync` rebases and returns `needs_judge` after conflict resolution.

A question ends its run. Relay its exact ID and text; restart the named `next_workflows` workflow with the owner's verbatim `answers` JSON. Follow `back.to` for a step back. A stop or rejected start is not a pass. Restart and fix limits default to 3; only the owner's words in `continue` authorize a start that reaches the restart limit. The installed `dev-runs` skill gives the handoff procedure.

## Why the parts fit together

`<standards>/concerns.json` maps ten qualities—structure, data, contract, security, ui, cost, test, delivery, scope and readability—to their principles and phase criteria. Every numbered principle has exactly one owner, counting `every_quality` as one shared assignment. Every review document except `review/README.md` must be mapped. `devkit concerns check` validates this and prepare stops on violations. The machine derives writer/judge materials from this map. Common judgment rules live in `review/README.md`; owner authority lives in `authority.md`.

Each quality has one `review/<quality>.md` with Decisions, Code, Blocking and Not reported sections. Plan judges choices and the skeleton; code judges bodies and conformance to passed choices. Agreed choices are carried by reference rather than judged again.

Decision records `<decisions>/D-<id>.md` hold repository decisions. IDs derive from branch and draft key using six lowercase hexadecimal characters, extending on collision; units and topics use the same scheme with `U-` and `T-`. Re-proposals retain kept keys' IDs and remove withdrawn proposed files. Agreed records are replaced through `supersedes`. Dependency traversal identifies affected decisions, documents and units. Passed owner/principle plan rows also become agreed decision records; provisional rows remain in the plan and PR body.

Plan settles structure before detail. New structure rows explain kind, reasons for change, volatility and callers with evidence or `unknown`. After structure passes, detail may change signatures but cannot add, remove or move files. Code judgment receives uncovered changed files, folder spread and ancestor siblings. Structural defects on the change's path block even if pre-existing; defects elsewhere are deferred. Jev selects qualities from each file's diff, using each quality's `routing` description and `routing_threshold` in `concerns.json`; when unavailable, `devkit judge route` runs the Codex CLI in a read-only sandbox. Configured globs and plan rows also select judges. Scope always runs; source/test changes also select test and readability.

Before pushing, PR commits `.devkit/backlog/<encoded-branch>.json`: deferred and carried findings, mechanical-check candidates and accepted objections, with their source judgments. Backlog files are excluded from judged-change identity. Identity otherwise preserves file names and every added/removed line, including whitespace, while ignoring hunk positions. `retro`, on a `retro/<name>` branch from the base, proposes work units, check additions and standard edits from the backlog. Its machine gate and independent judge precede an approval question. Approved changes and deletion of consumed backlog files share one commit.

## Repository settings

Edit `.devkit/config.json`; `devkit config` prints effective settings. The authoritative defaults and validation are in `internal/config`, and installed values are in `kit/.devkit/config.json`.

- `language`: owner-facing prose; default `ja`.
- `standards`, `docs`, `decisions`, `units`, `topics`: default `.devkit/standards`, `docs`, `.devkit/decisions`, `.devkit/units`, `.devkit/topics`.
- `checks`: named shell commands with `command`, `timeout_seconds` and optional `paths`. No checks are installed by default; add the repository's real checks. Omitted paths run every time; an empty paths array selects none. Exit 0 passes, 75 retries once as an environment error, 77 means not run, and other exits fail. Timeouts are environment errors.
- `skeleton_check`: names of checks to run on the skeleton; empty by default.
- `qualities`: path globs for the fixed qualities. Routing and plan rows also select judges; the concern map controls their plan/code phases.
- `decision_paths`: `{paths, quality, ask}` detection rules. Defaults include dependencies, migrations, infrastructure, CI, environment examples, workflows, standards and `.devkit/config.json`. Before/after check configurations are combined so deleting a check does not bypass it.
- `limits.restarts`, `limits.fixes`: default 3 each.

Missing settings use defaults; quality and limit entries merge individually, while other supplied arrays replace defaults. Globs support `**`, `*` and `?`. Repository-specific facts and conventions belong in these settings and standards.

## Test the kit

From this source checkout:

```sh
mise exec -- go build ./...
mise exec -- go vet ./...
mise exec -- go test ./...
scripts/test-kit.sh
```

`scripts/test-kit.sh [workflow ...]` builds both executables, initializes a temporary repository under `.local/tmp/`, validates workflows, checks principle ownership, tests a fresh installation, then copies source-only executed fixtures and tests them. Its isolated Archon home uses `$HOME/.archon/plugins`, overridable with `KIT_ARCHON_PLUGINS`. Every workflow has an executed main path: bash runs for real and AI is stubbed. PR publication is stubbed; sync exercises a real local rebase conflict. `cmd/devkit-fixture` supplies test-only setup and file writes and is absent from the shipped binary. The plumbing test rejects YAML/prompt calls to unregistered commands.

See the [workflow guide](kit/.archon/workflows/dev/README.md) for artifacts and execution contracts, and [design rationale](docs/design.md) in Japanese. Update these explanations alongside their authoritative code, YAML, schemas and standards.

## Current limits

There are no dependency facts from analysis tools and no narrowing of related decisions in `refine`; current materials use Git facts and full decision records. All workflow AI nodes use Claude; `devkit judge route` and `devkit refine reflect` run the Codex CLI with `--sandbox read-only` only when Jev cannot answer. If both are unavailable, routing selects every routable quality and reflection fails. Pull request publication supports GitHub through `gh`. Live AI behavior and external PR publication are outside fixture coverage.
