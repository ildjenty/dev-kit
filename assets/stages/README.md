Stage definitions describe the inputs, decisions, artifacts, and completion conditions for the stages selected by the plan.

A stage definition is Markdown named `<stage_id>.md`, with an initial `# <name>` and the following headings in order: `## Inputs`, `## Decisions`, `## Deferred to later stages`, `## Irreversible items`, `## Artifacts`, and `## Completion conditions`. The filename is the ID. These headings are authoring conventions; devkit supplies the entire stage document body without interpreting its sections. References such as `principles/…`, `stages/…`, `examples/…`, `plans/…`, `templates/…`, `rules.md`, `requirements.md`, and `structure.json` are logical names resolved through `[sources.<name>]` in `devkit.toml`. Briefs map these names to actual paths. `schemas/…` identifies binary-owned output schemas supplied in the brief. Patterns suggest stage arrangements. The task plan selects instance IDs and definition IDs, order, check names, gates, limits, and transitions, subject to the configured loop rules. Check runners and criteria come from devkit.toml.

- `stages/purpose.md`: 0 Purpose.
- `stages/scope.md`: 1 Scope.
- `stages/structure.md`: 2 Structure.
- `stages/contract.md`: 3 Contract.
- `stages/steps.md`: 4 Steps.
- `stages/implement.md`: 5 Implementation.
- `stages/edit.md`: Document editing.
- `stages/verification.md`: Verification.
- `stages/research.md`: Research.
- `stages/prototype.md`: Prototype.
- `stages/handoff.md`: Handoff.
- `stages/retrospective.md`: Retrospective.

The `transitions.done` value is a stage ID to continue, `null` to finish, or `"replan"` to have the planner rebuild the remaining plan after that stage completes. The name `replan` is reserved and cannot be a stage ID.

## Work inputs and submission

Use the task, human-approved completion conditions with IDs, `requirements.md`, frozen norms, plan limits, relevant records from `structure.json`, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from adopted previous stages. Every work brief references a work-history file fixed at instruction issue time; stage definitions determine how to use it. Fix rounds also use previous findings and their IDs, required `command:` check output, and pre-commit output. Do not require outputs from stages absent from the plan. Pass previous outputs and the conditions and sources attached to decisions and temporary state unchanged; agents and humans assess whether those conditions hold.

Write completion conditions in prose. For each condition, select a check run by a verification command, a judge, or a judgment command, or a human gate; checks need not be mechanical. Verify conditions during requirements definition through judges and human gates. The plan judge assesses the suitability and coverage of verification, and the human reviews it during plan approval; devkit does not enforce a formal assignment of verification to every condition.

The brief includes the current stage definition, the criteria documents selected by the checks being judged, `rules.md`, `requirements.md`, and relevant structure records. Other norms are listed by logical name and frozen snapshot path under `snap/`; read those paths when needed instead of norms being edited in the working tree. Round norms come from the round-start snapshot; plan checks and detection tests use committed norms from the work checkout's HEAD fixed when the instruction is issued. Requirements, structure records, detection pass indexes, and runtime records are outside norm identity; updating them does not invalidate judgments.

Use `schemas/plan.json` for plans, `schemas/progress.json` for termination blocks, and `schemas/structure.json` for structure records. The brief supplies self-contained schemas, including all referenced definitions and the supported note kinds. Use the actual working-tree paths shown in the brief for `requirements.md` and `structure.json`; create a missing product at its indicated path. Write the current work instruction's `instruction_id`, work performed, confirmed and unconfirmed matters, obstacles, termination kind, and resume conditions, including unreadable inputs and their effects. Temporary state includes its verification method, source, and invalidation conditions. Submit the JSON file with `devkit submit <file>`, then follow `devkit next`; follow the installed devkit skill for commands and pause/resume. A completion claim is a report, not a judgment or a completed stage.

When claiming completion, fill `report` according to this stage's selected report JSON Schema supplied in the brief. When asking the human, use `termination_kind: "confirmation-required"` and fill `confirmation` with the verbatim question (`text`), `options`, `recommendation`, `recommendation_reason`, and material references (`evidence`). Leave the recommendation and reason empty when no recommendation is supported. A question or stop may include a partial report conforming to the supplied fields; only a completion claim requires the full report. Schema conformance is not evidence of completion.

Workers of split-stage units follow the optional `unit_report` schema for their own reports. Devkit preserves each report with its unit ID for judgment after merging; do not rewrite unit reports into a stage report.

## Decisions for later work

Runtime records stay under the fixed, untracked `.devkit/run/` directory. Preserve decisions needed by future tasks in `requirements.md` for repository requirements, `structure.json` for modules, responsibilities, placement, and dependencies, or `rules.md` for shared difficult-to-change decisions with enforcement and grounds. Runtime notes and previous-stage outputs alone do not preserve those decisions for later tasks. Changes to `rules.md` and other norms require confirmation as loop edits under the configured approval policy; requirements and structure updates are work outputs assessed by stage checks.

## Completion finalization

Stage completion requires all required checks to be `PASS` for the returned target version and frozen norms, satisfaction of the detection requirements in `examples/README.md`, and devkit's successful finalization at a matching committed target. The judgment rules are in the judgment request template. Select verification through the plan; this definition adds no checks.

Devkit aggregates required checks and finalizes completion; agents do not commit. It commits the judged snapshot without `--no-verify`, using the repository's existing pre-commit hook. A failed commit returns its output to a fix round. Devkit compares the target before committing and the committed tree afterward; a mismatch requires renewed checks, and a further change after one refresh requires human confirmation. Devkit commits on the current branch and creates no branches. The commit message is the worker's `commit_message` from the progress block, written following the git rules (`git` source, default `.devkit/git.md`); without it, devkit uses the `[git] commit_message` template. Empty-commit behavior follows `[git]` in `devkit.toml`. If `allow_empty_commit` is false, an unchanged stage records completion at the current HEAD. Completion requires a matching committed target. The Stop hook does not finalize completion.

## Split stage work

For a stage with `split: true` and planned `units`, launch the returned assigned agents once each. Each worker edits only the absolute detached worktree and unit scope in its brief, then returns a JSON-only progress block in its final message; it does not run `devkit submit`. SubagentStop stores the result. Devkit fixes each unit tree as a SHA and merges the inputs in plan order into the work checkout without a completion commit. One stage round judges the merged tree using the cumulative diff from stage start. Conflicts return to the stage session worker in a fix round with the conflicting files and units. Completion commits follow the ordinary stage procedure.

Pause issues no new units and preserves results from running workers. Abort and completion remove unchanged unit worktrees and report retained changed worktrees for the human to manage with git. Do not discard them automatically.

Stage assignees and judge runners are role IDs defined in `devkit.toml`. Agent-backed work roles, including the default `worker` (agent `devkit-worker`), run as one unit in a devkit-created worktree and return the progress block as the subagent's final message; the session agent launches them and waits. A work role with an empty `agent` means the session itself works in the checkout. Split-stage unit roles default to `worker`. Fix rules can reassign the next round within the same StageRun; cumulative differences and limits remain shared. Read-only source and rejected loop-edit undo rounds preserve the prior assignee. See `plans/README.md` for the rule shape and role constraints.

Stage instance IDs are unique within a plan. `definition` selects this catalog; when omitted, the instance ID is also the definition ID. The same definition may be used repeatedly. Transitions, start_stage, return destinations, unit IDs, and fix rules refer to instances. Output selections may name an instance or select every instance of a definition. History and counts remain per instance; retrospective also groups counts by definition.
