# dev-kit

dev-kit is a tool for running a repository-specific development loop inside Claude Code or Codex CLI sessions.

dev-kit is this repository, kit is the name for everything it provides, and `devkit` is the binary. The repository owner composes a loop template from stages, checks, and human gates. Devkit enforces the chosen order, independent judgments, approvals, and records. The repository owns and maintains the installed principles, stages, aspects, plan templates, brief templates, and rules; the binary owns the record schemas and host configuration fragments.

## Installation

Install Go (the version required by `go.mod`) and Git, then clone this repository and run the installer. It builds `devkit` and places it in a directory on `PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
/path/to/dev-kit/install.sh
```

Usage: `/path/to/dev-kit/install.sh [--bin DIR]`. `--bin DIR` sets the directory for `devkit` (default: `$HOME/.local/bin`); it must be on `PATH`.

Then run `devkit init` in each repository that uses devkit:

```sh
cd /path/to/your-repository
devkit init
```

`devkit init` preserves existing repository settings and assets and merges missing kit settings, including deny defaults. After showing the changes and receiving confirmation, it installs and commits the files; `--no-commit` skips that commit. Before creating a branch or records, `devkit start` requires an existing HEAD and checks that existing configured norm paths, `devkit.toml`, and agent definitions are committed and unmodified. Commit the installation changes first if start refuses them. Init writes `kit_version = "v2"`, which identifies record-format compatibility rather than an installed norm copy.

The `default` and `fix` templates require `command:test`, `test`, and `diff` during implementation, and `command:test`, `test`, and `harness` during retrospective. The `docs` template requires `wording` during implementation and only `harness` during retrospective. If `command:test` is not registered, its check returns a fixable failure: the first work creates the tests and verification method and registers it in `devkit.toml`. A human confirms the registration before it takes effect in the next round. A registration looks like this:

```toml
[commands.test]
executable = "./scripts/test"
args = []
timeout_seconds = 120
```

```json
{ "aspect": "command:test", "runner": "command:test", "required": true }
```

To remove devkit from a repository, run `devkit uninstall` there. It shows the changes and asks for confirmation (`--yes` skips it), then removes runtime state, the skill link, and installed norms and agent definitions that still match the bundled defaults. It strips installed fragments from host settings and `.gitignore`, and removes `devkit.toml` only when it contains no repository-specific settings. Modified files and repository products such as requirements and structure records are preserved; retained configuration and modified installed files are listed. To remove the binary, delete `devkit` from the `--bin` directory.

In Codex, trust the project interactively and trust each project hook through `/hooks`. In Claude Code, accept workspace trust. Start a new host session and run `devkit doctor`, then use `/devkit` to begin. Run `devkit help` for command arguments. See [install](docs/architecture/modules.md#install) and [doctor](docs/architecture/modules.md#doctor-の確認項目) for the design.

Tools such as Go, npm, and pip that write caches outside the repository may fail when agents run them inside the host sandbox; host sandbox settings can allow those locations, while devkit's own verification commands run outside the sandbox.

A repository can opt into parallel stage work with `split: true` in a template and `units` in its plan (each with `id`, `description`, and `scope`); `[parallel] max_workers` defaults to 3 and must be at least 1. Devkit creates detached unit worktrees under the ignored `.devkit/worktrees/<instruction_id>/`, issues `devkit-worker` instructions, fixes accepted results by SHA, and merges them into the work checkout for one ordinary stage round. Conflicts return to the session worker for a fix round; judgments, gates, loop-edit confirmation, limits, back/replan, and questions remain at stage level. Pause stops new instructions; abort and completion remove unchanged unit worktrees and report changed ones that remain. The bundled templates do not split stages.

## Reading order

1. [docs/architecture.md](docs/architecture.md): Overview of the structure and behavior, the work flow and the scope of the guarantees.
2. [docs/design.md](docs/design.md): Basic policy and the roles of the seven parts.
3. [principles/README.md](assets/principles/README.md): The principles that [check](docs/parts/check.md) (the part for evaluation and judgment) uses as criteria, and how to write a principle.

The canonical design is [docs/architecture.md](docs/architecture.md) and [docs/architecture/](docs/architecture/). If `docs/parts/` or the documents directly under `docs/` disagree with it, the canonical design wins. The work procedure follows the [flow](docs/architecture/flow.md).

Pausing and resuming, and the Stop continuation limit, follow [interruption](docs/architecture/guarantees.md#中断).

The design documents under `docs/` and the principles under `assets/principles/` are written in Japanese. Everything devkit distributes is under `assets/`; `docs/` is for developing dev-kit and is not distributed.

## Files

- [docs/architecture.md](docs/architecture.md): Design overview.
- [docs/architecture/](docs/architecture/): Design details.
  - [flow.md](docs/architecture/flow.md): The flow from start to completion.
  - [data.md](docs/architecture/data.md): Records and state.
  - [modules.md](docs/architecture/modules.md): Modules and their dependencies.
  - [layout.md](docs/architecture/layout.md): Directory layout.
  - [hooks.md](docs/architecture/hooks.md): The five hooks.
  - [guarantees.md](docs/architecture/guarantees.md): The prevention mechanism for each failure.
  - [examples.md](docs/architecture/examples.md): Minimal examples of a template, plan, settings, aspect and pre-commit.
- [docs/design.md](docs/design.md): Basic policy and the list of parts.
- [docs/parts/](docs/parts/): The contract of each part.
- [docs/loop-plan.md](docs/loop-plan.md): The format and rules for loop templates (definitions composed by the user) and per-task plans.
- [docs/stages.md](docs/stages.md): Candidate stages for a template.
- [docs/checks.md](docs/checks.md): Candidate [aspect files](docs/architecture/modules.md#check) (files that state check's judgment criteria).
- [docs/rules.md](docs/rules.md): The design-level list of rules imposed on target code and documents, and their enforcement. The canonical version in a using repository follows [Rules](docs/architecture/data.md#規則と強制手段-rules).
- [docs/repository.md](docs/repository.md): How the using repository's layout maps to the execution mechanisms.
- [docs/meta-loop.md](docs/meta-loop.md): The procedure for improving the loop itself.
- [assets/](assets/): Everything devkit distributes. `devkit init` installs repository-owned norms at their configured locations, defaulting to `.devkit/principles/`, `.devkit/stages/`, `.devkit/checks/`, `.devkit/plans/`, `.devkit/templates/`, and `.devkit/rules.md`. Requirements and structure records are repository products. Execution records stay in the fixed, untracked `.devkit/run/` directory; skills and subagent definitions are installed in their host locations.
  - [principles/](assets/principles/): The principles, split into files by where they apply.
