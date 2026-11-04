The `default.toml` classification table supplies default actions. The binary supplies human confirmation categories and lesson destination defaults. Repository settings belong in `devkit.toml`.

`[router] human_confirmation` replaces the default list, including when explicitly empty. Known classification names are:

- `dependency_changed`: A dependency manifest or lockfile diff.
- `target_changed_again`: Another target change after refreshing the target once around commit.

Loop-edit confirmation is governed only by `[approval] meta_loop_edit = "confirm" | "allow"` (default `confirm`), both before a round and before a completion commit. It covers norms, `devkit.toml`, host settings, instruction files, skills, and the active pre-commit configuration, excluding requirements, structure records, detection indexes, and runtime records. It cannot be removed through `[router] human_confirmation`.

`[plan_check] aspect = "plan"` and `runner = "judge"` select the plan-check aspect and runner. A registered judgment command runs directly; otherwise devkit issues a subagent instruction.

`check_results` holds fixed actions for aggregate results. Roles are configured under `[roles.<id>]` in `devkit.toml`. `[lessons] destinations` in `devkit.toml` sets the priority order for lesson destinations, defaulting to `hook_lint_type`, `plan_template`, `aspect`, `principle`, and `skill`. `fix`, `ask`, `back`, and `done` use the same vocabulary as template transitions. Router checks limits, progress, human answers including `back`, termination kinds, and completion conditions; this mapping alone does not finalize completion. Runners are assigned per check, not here.

The default aggregate mappings are interpreted as `fix` for a fixable `FAIL`, `ask` for `NOT ASSESSED` when evaluation is unavailable, and `done` for `PASS` that meets completion conditions. A `back` interpretation of a human answer to a stage question follows the back and replan path. Aspect names do not trigger a special return; limits and progress are additional routing inputs.

A blocking `premise_broken` finding asks the human and recommends `back`; the finding itself never returns to an earlier stage. Only fix transitions can select a different assignee by ordered rules; round limits take precedence.
