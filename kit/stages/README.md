Stage definitions describe the inputs, decisions, artifacts, and completion conditions for the stages selected by the plan.

A stage definition is Markdown named `<stage_id>.md`, with an initial `# <name>` and the following headings in order: `## Inputs`, `## Decisions`, `## Deferred to later stages`, `## Irreversible items`, `## Artifacts`, and `## Completion conditions`. The filename is the ID; each section is machine-readable through the next second-level heading. Code-formatted references starting with `principles/` or a kit directory such as `schemas/`, `stages/`, or `checks/` are relative to the installed kit copy root. In the source repository, kit directories are under `kit/`, while `principles/` is at the repository root. The plan template and plan determine adopted stages, order, checks, runners, gates, limits, and transitions.

- `purpose.md`: 0 Purpose.
- `scope.md`: 1 Scope.
- `structure.md`: 2 Structure.
- `contract.md`: 3 Contract.
- `steps.md`: 4 Steps.
- `implement.md`: 5 Implementation.
- `research.md`: Research.
- `prototype.md`: Prototype.
- `initialize.md`: Initialization.

## Work inputs and submission

Use the task, completion conditions with IDs, frozen norms, plan limits, placement guides, difficult-to-change decisions in `.devkit/rules.md` and the plan, available checks, and outputs from adopted previous stages. Fix rounds also use previous findings and their IDs, required `command:` check output, and pre-commit output. Do not require outputs from stages absent from the plan. Pass previous outputs and the conditions and sources attached to decisions and temporary state unchanged; agents and humans assess whether those conditions hold.

Use `schemas/plan.json` for plans and `schemas/progress.json` for termination blocks. Write the current work instruction's `instruction_id`, work performed, confirmed and unconfirmed matters, obstacles, termination kind, and resume conditions, including unreadable inputs and their effects. Temporary state includes its verification method, source, and invalidation conditions. Submit the JSON file with `devkit submit <file>`, then follow `devkit next`; see `skills/devkit/SKILL.md` for commands and pause/resume. A completion claim is a report, not a judgment or a completed stage.

## Completion finalization

Stage completion requires all required checks to be `PASS` for the returned target version and frozen norms, detection test passes for required aspects and their assigned judges, and devkit's successful target comparison and `git commit`. See `checks/README.md` for judgment and detection requirements. Select verification through the plan; this definition adds no checks.

Devkit aggregates required checks and finalizes completion; agents do not commit. It commits the judged snapshot without `--no-verify`, using the repository's existing pre-commit hook. A failed commit returns its output to a fix round. Devkit compares the target before committing and the committed tree afterward; a mismatch requires renewed checks, and a further change after one refresh requires human confirmation under `router/README.md`. Completion is not recorded without a matching committed target. The Stop hook does not finalize completion.
