Stage definitions describe the inputs, decisions, artifacts, and completion conditions for the stages selected by the plan.

A stage definition is Markdown named `<stage_id>.md`, with an initial `# <name>` and the following headings in order: `## Inputs`, `## Decisions`, `## Deferred to later stages`, `## Irreversible items`, `## Artifacts`, and `## Completion conditions`. The filename is the ID; each section is machine-readable through the next second-level heading. References such as `principles/…`, `stages/…`, `checks/…`, `plans/…`, `templates/…`, `rules.md`, `requirements.md`, and `structure.json` are logical names resolved through `[paths]` in `devkit.toml`. Briefs map these names to actual paths. `schemas/…` identifies binary-owned output schemas supplied in the brief. The plan template and plan determine adopted stages, order, checks, runners, gates, limits, and transitions.

- `stages/purpose.md`: 0 Purpose.
- `stages/scope.md`: 1 Scope.
- `stages/structure.md`: 2 Structure.
- `stages/contract.md`: 3 Contract.
- `stages/steps.md`: 4 Steps.
- `stages/implement.md`: 5 Implementation.
- `stages/research.md`: Research.
- `stages/prototype.md`: Prototype.
- `stages/retrospective.md`: Retrospective.

The `transitions.done` value is a stage ID to continue, `null` to finish, or `"replan"` to have the planner rebuild the remaining plan after that stage completes. The name `replan` is reserved and cannot be a stage ID.

## Work inputs and submission

Use the task, human-approved completion conditions with IDs, `requirements.md`, frozen norms, plan limits, relevant records from `structure.json`, difficult-to-change decisions in `rules.md` and the plan, available checks, and outputs from adopted previous stages. Every work brief references a work-history file fixed at instruction issue time; stage definitions determine how to use it. Fix rounds also use previous findings and their IDs, required `command:` check output, and pre-commit output. Do not require outputs from stages absent from the plan. Pass previous outputs and the conditions and sources attached to decisions and temporary state unchanged; agents and humans assess whether those conditions hold.

Write completion conditions in prose. For each condition, select a check run by a verification command, a judge, or a judgment command, or a human gate; checks need not be mechanical. Verify conditions during requirements definition through judges and human gates. The plan judge assesses the suitability and coverage of verification, and the human reviews it during plan approval; devkit does not enforce a formal assignment of verification to every condition.

The brief includes the current stage definition, the aspect being judged and its referenced principles, `rules.md`, `requirements.md`, and relevant structure records. Other norms are listed by logical name and frozen snapshot path under `snap/`; read those paths when needed instead of norms being edited in the working tree. Round norms come from the round-start snapshot; plan checks and detection tests use committed norms from the work branch HEAD fixed when the instruction is issued. Requirements, structure records, detection pass indexes, and runtime records are outside norm identity; updating them does not invalidate judgments.

Use `schemas/plan.json` for plans, `schemas/progress.json` for termination blocks, and `schemas/structure.json` for structure records. The brief supplies self-contained schemas, including all referenced definitions and the supported note kinds. Use the actual working-tree paths shown in the brief for `requirements.md` and `structure.json`; create a missing product at its indicated path. Write the current work instruction's `instruction_id`, work performed, confirmed and unconfirmed matters, obstacles, termination kind, and resume conditions, including unreadable inputs and their effects. Temporary state includes its verification method, source, and invalidation conditions. Submit the JSON file with `devkit submit <file>`, then follow `devkit next`; follow the installed devkit skill for commands and pause/resume. A completion claim is a report, not a judgment or a completed stage.

## Decisions for later work

Runtime records stay under the fixed, untracked `.devkit/run/` directory. Preserve decisions needed by future tasks in `requirements.md` for repository requirements, `structure.json` for modules, responsibilities, placement, and dependencies, or `rules.md` for shared difficult-to-change decisions with enforcement and grounds. Runtime notes and previous-stage outputs alone do not preserve those decisions for later tasks. Changes to `rules.md` and other norms require confirmation as loop edits under the configured approval policy; requirements and structure updates are work outputs assessed by stage checks.

## Completion finalization

Stage completion requires all required checks to be `PASS` for the returned target version and frozen norms, satisfaction of the detection requirements in `checks/README.md#detection-tests`, and devkit's successful finalization at a matching committed target. See `checks/README.md` for judgment and detection requirements. Select verification through the plan; this definition adds no checks.

Devkit aggregates required checks and finalizes completion; agents do not commit. It commits the judged snapshot without `--no-verify`, using the repository's existing pre-commit hook. A failed commit returns its output to a fix round. Devkit compares the target before committing and the committed tree afterward; a mismatch requires renewed checks, and a further change after one refresh requires human confirmation. Commit messages, branch names, and empty-commit behavior follow `[git]` in `devkit.toml`. If `allow_empty_commit` is false, an unchanged stage records completion at the current HEAD. Completion requires a matching committed target. The Stop hook does not finalize completion.
