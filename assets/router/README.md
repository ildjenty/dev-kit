The `default.toml` classification table supplies default actions and role vocabulary. Human confirmation categories and lesson destinations have one source of defaults in `internal/config`, applied through `router.Table.WithConfig`. Repository settings belong in `devkit.toml`.

`[router] human_confirmation` replaces the default list, including when explicitly empty. Known classification names are:

- `dependency_changed`: A dependency manifest or lockfile diff.
- `meta_loop_edit`: Changes to norms, `devkit.toml`, host settings, instruction files, skills, and the active pre-commit configuration. Requirements, structure records, detection indexes, and runtime records are excluded. Stage declarations do not exempt loop edits. `[approval] meta_loop_edit = "allow"` permits them without confirmation.
- `target_changed_again`: Another target change after refreshing the target once around commit.

`check_results` holds default actions for aggregate results, and `roles` holds role vocabulary. `[lessons] destinations` in `devkit.toml` sets the priority order for lesson destinations, defaulting to `hook_lint_type`, `plan_template`, `aspect`, `principle`, and `skill`. `fix`, `ask`, `back`, and `done` use the same vocabulary as template transitions. Router checks limits, progress, broken premises, termination kinds, and completion conditions; this mapping alone does not finalize completion. Runners are assigned per check, not here.

The default aggregate mappings are interpreted as `fix` for a fixable `FAIL`, `ask` for `NOT ASSESSED` when evaluation is unavailable, and `done` for `PASS` that meets completion conditions. `back` for broken premises and count limits are not determined by the judgment value alone.
