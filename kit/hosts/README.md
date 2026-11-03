Stores host settings, judge and planner subagent definitions, and rules fragments.
Procedures for using devkit belong in kit/skills/; event JSON conversion belongs in internal/host/event.
For placement, see `docs/architecture/modules.md#kit`; for installation, see `docs/architecture/modules.md#install`.

- `claude/settings.json` contains hook, sandbox, and permissions fragments to append to `.claude/settings.json`.
- `claude/agents/` contains read-only judge and planner definitions placed in `.claude/agents/`.
- `codex/hooks.json` and `codex/config.toml` contain fragments to append to the same-named files in `.codex/`.
- `codex/agents/` and `codex/rules/` contain definitions placed in the same-named directories in `.codex/`.
- `deny.toml` contains the default `[deny]` fragment for `devkit.toml`. `rules` applies to all bound sessions; `judge` lists write tools forbidden only to devkit judge and planner subagents.

The hook command `devkit hook <event>` registers the kit binary during installation. The installer shows the diff and appends entries without overwriting existing ones. Repository-wide deny entries are limited to the two Edit restrictions for records and results; PreToolUse handles work-bound operation and subagent restrictions.
