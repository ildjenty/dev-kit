Stores the default classification table used by router.
Classification and decision logic belong in internal/router; repository additions belong in the consuming repository's devkit.toml.
For placement, see `docs/architecture/modules.md#kit`; for decisions, see `docs/architecture/modules.md#router`.

`human_confirmation` in `default.toml` lists classification names that require human confirmation.

- `dependency_changed`: A dependency manifest or lockfile diff.
- `undeclared_meta_loop_edit`: Changes to the loop itself in a stage that has not declared meta-loop edits.
- `target_changed_again`: Another target change after refreshing the target once around commit.

`check_results` holds default actions for aggregate results, `roles` holds role vocabulary, and `lessons.destinations` holds the priority order for lesson destinations. `fix`, `ask`, `back`, and `done` use the same vocabulary as template transitions. Router checks limits, progress, broken premises, termination kinds, and completion conditions; this mapping alone does not finalize completion. Runners are assigned per check, not here.

The source of truth does not specify a table storage format or classification names, so TOML and the names above are used. The default aggregate mappings are interpreted as `fix` for a fixable `FAIL`, `ask` for `NOT ASSESSED` when evaluation is unavailable, and `done` for `PASS` that meets completion conditions. `back` for broken premises and count limits are not determined by the judgment value alone.
