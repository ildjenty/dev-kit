Plan templates are JSON files named `<name>.json` in `plans/`, maintained by the repository owner after `devkit init`. Select one with `devkit start --plan <name>`. Omitting `--plan` uses `default_plan` in `devkit.toml`, whose default is `default`.

Templates follow `schemas/plan-template.json`; task-specific plans follow `schemas/plan.json`, and planner responses follow `schemas/planner-result.json`. Templates define stages, assignees, checks and runners, gates with reasons, limits, and transitions. Gate reasons are free text; use the applicable classification name from `devkit.toml` or its defaults.

## Checks

Each check has a name (`aspect`), a runner, and a required status. A judge-run check also selects its review criteria and its detection examples from knowledge sources:

```json
{
  "aspect": "test",
  "runner": "judge",
  "required": true,
  "criteria": [
    {"source": "principles", "select": "review/test.md"},
    {"source": "principles", "select": "testing.md"}
  ],
  "examples": {"source": "examples", "select": "test/spec.json"}
}
```

`criteria` lists the review document from `principles/review/` and every document that review names as part of its criteria; see `principles/review/README.md`. `examples` selects the detection specification; see `examples/README.md`.

A `command:<name>` check runs the registered verification command and has no criteria or examples. If `command:<name>` is absent from the round-start `devkit.toml`, the check returns `FAIL` with an `auto-fixable` finding: "Verification command `<name>` is not registered. Create a verification method and register it in `[commands.<name>]` in `devkit.toml`." Registration is a loop edit, follows the human confirmation policy, and takes effect from the next round's snapshot. If a command changes the tree, its finding also instructs the worker to add generated output paths to the repository's Git ignore rules.

The `transitions.done` value is a stage ID to continue, `null` to finish, or `"replan"` to have the planner rebuild the remaining plan after that stage completes. Remaining work follows the completed stage run's bound plan and its transition order, including when a revised proposal is resubmitted. The name `replan` is reserved and cannot be a stage ID.

- `default`: purpose → scope → structure → contract → steps → implement → verification → retrospective, with a gate at scope. Optional candidates are research, prototype, and handoff. Implement requires `command:test`, `test`, and `diff`; verification requires `command:e2e` and `behavior`.
- `fix`: implement → verification → retrospective. Optional candidates are research, prototype, and handoff. Implement requires `command:test`, `test`, and `diff`; verification requires `command:e2e` and `behavior`.
- `docs`: implement → retrospective. Implement requires `wording`.

In `default`, the first plan's completion conditions concern agreement on requirements and completion conditions. During purpose, clarify requirements with the human, record them in `requirements.md`, and output an agreed proposal for completion conditions. After purpose completes, the planner rebuilds the remaining plan using those conditions, and the human approves it. The `fix` and `docs` templates do not use this initial agreement flow.

Retrospective requires `command:test`, `test`, and `harness` in `default` and `fix`, and only `harness` in `docs`. Judge-run checks use the `judge` role (agent `devkit-judge`); `command:test` and `command:e2e` checks use the registered commands of the same names. Each stage is assigned to the `worker` role (agent `devkit-worker`).

Verification runs each completion condition's usage scenario end to end through the user's entry point. When `[commands.e2e]` is not registered, the worker creates an end-to-end script and registers it, the same path as `command:test`. The `behavior` check judges whether each scenario is confirmed by that run. Handoff writes the PR description for reviewers following `templates/pr.md.tmpl` and is checked by `wording`; it performs no git operations. Insert it before retrospective when the work ends in a pull request. Stage inputs in the bundled templates include the `git` source (`.devkit/git.md`), which defines branch, commit message, and integration rules.

Build the smallest plan for what changes. Remove unrelated stages, select needed optional stages, write completion conditions in prose with IDs, and assign each check a target and completion condition IDs. Give each completion condition a way to verify it: a check run by a verification command, a judge, or a judgment command, or a human gate. Checks are not necessarily mechanical. Verify conditions during requirements definition through judges and human gates. The plan judge assesses the suitability and coverage of verification, and the human reviews it during plan approval; devkit does not enforce a formal assignment of verification to every condition. Preserve task-specific irreversible decisions in the plan; shared decisions and their enforcement and grounds belong in `rules.md`. Use `requirements.md` as the repository's requirements source. Stages changing UI, data, or external communication need behavior checks. Required judge-run checks follow `examples/README.md` for detection tests. If requirements are ambiguous, list unresolved points in the optional `open_questions` string array instead of guessing.

Removing stages, including initial stages, adding stages outside the template candidates, removing gates, changing transitions, removing required checks, or changing a check's `criteria` or `examples` selections are deviations; mark applicable findings `needs-decision` using `schemas/finding.json`. Declared external operations require human approval even without deviations; declarations identify kind, target, input hash, definition hash, and executable content hash. Propose replanning when completion conditions or premises need changing; do not silently treat a proposal as the active plan.

With the default `[approval] plan = "requirements"`, human approval is required for the first plan, any plan whose completion condition IDs or wording differ from the last human-approved version, any deviation, and any declared external operation. Replanning with identical completion conditions and no deviations, external operations, or open questions stays within the approved agreement and needs no repeated approval. `always` requires approval for every version; `deviation` requires it for deviations or declared external operations. A nonempty `open_questions` array always requires human approval, regardless of policy. Human-approved completion conditions define completion. Answers to `open_questions` return to the planner as rejection and replanning instructions, with no limit on reproposals.

The approval question includes the original human request, the task passed to `start`, completion conditions with IDs, adopted and omitted stages with deviations identified, required checks per stage, task-specific irreversible items, external operation declarations, and `open_questions`. Present devkit's generated question without replacing it with an agent summary. Follow the installed devkit skill for answers and external operations.

A template stage may opt into `split: true`. Its task plan may supply `units`, each with a unique plan-wide `id`, a `description`, and a prose `scope` naming the files or directories it owns. Units require a split stage assigned to a work role; without units, work follows the ordinary stage path. Adding or removing `split` is a deviation. The plan judge reports overlapping unit scopes; devkit checks ID uniqueness, not scope overlap. The bundled templates do not use splitting.

Devkit limits outstanding subagent instructions with `[parallel] max_concurrent` (default 3, minimum 1), creates detached worktrees keyed by instruction ID, and merges the fixed unit SHAs before the usual stage judgment. Independent unit work, judge checks after verification commands, and pre-judgment planner instructions may run concurrently within both the global limit and `[roles.<id>] max_concurrent` limits; count issued subagent instructions until accepted. Gates, loop-edit confirmation, limits, back/replan, and questions remain at stage level. Stages already completed in this work and absent from a rebuilt plan are not removed-stage deviations and are omitted from the approval question.

## Roles and correction rules

Assign `assignee`, judge-run check `runner`, and unit `role` using role IDs from `[roles.<id>]` in `devkit.toml`. Defaults are `worker` (agent `devkit-worker`), `judge` (agent `devkit-judge`), `planner` (agent `devkit-planner`), and `human`. Unit `role` defaults to `worker` and requires a work role with an agent. Human roles may be stage assignees; gates are always human. Judge and plan roles are read-only. `[planning] role` selects the planner and `[plan_check] runner` selects the independent plan judge. Agent names must start with `devkit-`.

A work assignee with an agent and no explicit split units runs in one devkit-created detached worktree as a single unit, using the same merge and judgment path as split work. The session agent orchestrates: it launches the returned subagents, waits for their results, presents questions to the human, and integrates the work following the git rules. A work role with an empty `agent` means the session itself works in the checkout. A split stage may use the default worker role. Merge conflicts return to the stage assignee: an agent-backed work role receives one repair unit in a detached worktree with the merge in progress and conflict markers present, and its fixed result is merged before the remaining unit SHAs. An empty-agent work role repairs conflicts in the session checkout.

Only `transitions.fix` accepts an ordered rule array instead of the current stage ID:

```json
"fix": [
  {"when": {"check": "diff", "failures_at_least": 2}, "assignee": "senior_worker"},
  {"when": {"human_rejected": true}, "assignee": "senior_worker"}
]
```

Conditions are combined with AND. `check` matches an aspect with FAIL or NOT ASSESSED in the current round, regardless of runner. `failures_at_least` counts rounds with any required FAIL in this StageRun, including command checks. `human_rejected` matches a rejected stage question. The first match changes the next correction round's assignee; no match returns to the stage's default assignee. Rules never change stage, comparison base, or cumulative diff. Limits take precedence. A limit question can offer `continue:<role>` for roles in the rules; only a human choice restarts the limit window. Undoing rejected loop edits or read-only source edits preserves the preceding assignee without applying rules.

Changing assignees, the fix rule list, or unit roles is a deviation shown with its template and plan values in the approval question. A judge's blocking finding may set `premise_broken: true`; devkit recommends `back` in a human question, and only the human's back answer follows the back/replan path.

Sources outside the repository are read-only: omit `writable` or set it to false. Devkit fixes their documents for each instruction; it does not create editable copies or write changes back. Apply improvements only to writable sources inside the repository, and record improvements to read-only sources as lessons for their maintainers.

## Reports, instructions, and context

A stage may select `report`, `unit_report`, and `human_prompt` as single knowledge selections (`source` and `select`). Reports select JSON Schema documents in norm sources, such as `templates/reports/implement.json`. A completion claim must match the selected report schema; questions and stops may contain partial reports. `unit_report` is optional and applies independently to each unit report, retained with its unit ID. Schema conformance is not evidence of completion. Human-assigned stages use the selected `human_prompt` document as the question.

Stage `delegations` is an array of document selections passed verbatim with provenance to applicable judges. Keep these standing delegations distinct from human-answer approvals. They cannot delegate plan or completion-condition changes, external operations, loop edits, limits, or interruption.

Stages, checks, and units in task plans may contain `instructions` arrays of strings. Templates cannot contain instructions. Include these instructions in the approval question and corresponding role briefs without reinterpretation. Instructions are supplementary and cannot narrow fixed criteria or judgment scope. A check may enable `brief_by_planner` to obtain additional guidance immediately before judgment; the bundled templates leave it disabled. Guidance, history selection, and reissues are bound to the round, aspect, and runner.

Stage and check `history` selects `none`, `stage_run`, `check`, or `all`. Defaults are `stage_run` for workers and `check` for judges. Bundled retrospective stages and harness checks select `all`. Supply history as a fixed index reference, not an inline transcript. Regardless of this setting, judges receive applicable command results, valid approvals, and cited evidence and references.

Stage, check, and unit `outputs` selects stage IDs whose adopted outputs are inline. Omitted or null means all adopted outputs; an empty array means none inline. Other outputs remain accessible through a fixed index. Adopted outputs come from the active execution path after the latest back or replan, using each completed StageRun's last adopted round. Superseded runs belong to history.

Check `schemas` lists binary schema names, without paths or `.json`, needed as input contracts. Supply their union with contracts referenced by the output schema, once each. The configured plan check defaults to `plan` and `plan-template`. Changes to stage report, unit report, human prompt, delegation, output, or history selections, or check history, schemas, outputs, or brief_by_planner are template deviations. Plan-authored instructions are not deviations. Units have no `after`; work depending on another result belongs in a later stage.
