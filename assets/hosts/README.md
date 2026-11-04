These JSON and TOML settings fragments and Markdown/TOML subagent definitions configure the supported hosts. `devkit init` installs them at the paths below. Procedures for using devkit are in `.agents/skills/devkit/SKILL.md`.

- `claude/settings.json` contains hook, sandbox, and permissions fragments to append to `.claude/settings.json`.
- `claude/agents/` contains the read-only judge and planner definitions and the worker definition placed in `.claude/agents/`. The default `worker` role uses `devkit-worker`, which performs stage work and split units in devkit-created worktrees while the session agent orchestrates.
- `codex/hooks.json` and `codex/config.toml` contain fragments to append to the same-named files in `.codex/`.
- `codex/agents/` and `codex/rules/` contain definitions placed in the same-named directories in `.codex/`.
- `deny.toml` contains the default `[deny]` fragment for `devkit.toml`. `rules` applies to all bound sessions; `judge` lists write tools forbidden only to devkit judge and planner subagents.

Subagents read `.devkit/run/work/<work_id>/brief/<instruction_id>.md`. Briefs include selected norm text and paths to the remaining norms in the fixed snapshot expansion under `snap/`; subagents read those norms from the supplied paths rather than the working tree. The runtime root `.devkit/run/` is fixed.

The hook command `devkit hook <event>` registers the kit binary during installation. The installer shows the diff and appends entries without overwriting existing ones. Repository-wide deny entries are limited to the two Edit restrictions for records and results; PreToolUse handles work-bound operation and subagent restrictions.
