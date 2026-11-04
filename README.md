# dev-kit

dev-kit is a tool for running a repository-specific development loop inside Claude Code or Codex CLI sessions.

dev-kit is this repository, kit is the name for everything it provides, and `devkit` is the binary. The repository owner composes a [loop template](docs/architecture/data.md#ループの雛形-plantemplate) (a combination of work stages, judgments and approvals). The kit provides the [seven parts](docs/design.md#部品), the principles, the [fixed assets](docs/architecture/modules.md#kit) (versioned assets) and `devkit`.

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

`devkit init` preserves existing repository settings and assets and merges missing kit settings, including deny defaults.

The bundled templates in `.devkit/plans/` use judge checks only, so they work in a repository that has no tests yet. Once the repository has tests or other mechanical checks, register them in `devkit.toml` and add `command:<name>` checks to your templates:

```toml
[commands]
test = { executable = "./scripts/test", args = [], timeout_seconds = 120 }
```

```json
{ "aspect": "command:test", "runner": "command:test", "required": true }
```

To remove devkit from a repository, run `devkit uninstall` there. It shows the changes and asks for confirmation (`--yes` skips it), then removes the kit copy, runtime state, the skill link and installed files that still match the kit, and strips the kit settings from host settings, `devkit.toml` and `.gitignore`. Files you changed are left in place and listed. To remove the binary, delete `devkit` from the `--bin` directory.

In Codex, trust the project interactively and trust each project hook through `/hooks`. In Claude Code, accept workspace trust. Start a new host session and run `devkit doctor`, then use `/devkit` to begin. Run `devkit help` for command arguments. See [install](docs/architecture/modules.md#install) and [doctor](docs/architecture/modules.md#doctor-の確認項目) for the design.

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
- [assets/](assets/): Everything devkit distributes. `devkit init` copies it into `.devkit/kit/<version>/` in the using repository.
  - [principles/](assets/principles/): The principles, split into files by where they apply.
