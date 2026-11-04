# Brief request templates

These complete Go `text/template` sources define the work, judgment, and planning requests in the installed kit copy's `templates/brief/` directory.

- `work.md.tmpl`: The current stage, task and plan, allowed and forbidden changes, verification, repository facts, frozen norms, previous outputs and findings, and command or pre-commit output. Return the progress block using `schemas/progress.json` and `devkit submit <file>`.
- `judgment.md.tmpl`: The judgment content and instruction ID. Return a JSON-only final message using the supplied schema (`schemas/judgment.json`, or `schemas/detection-result.json` for detection tests).
- `planner.md.tmpl`: The task, loop template, stage definitions, repository facts, frozen norms, available checks, and any current or remaining plan, human instructions, and previous findings. Return a JSON-only final message using `schemas/planner-result.json` with a plan following `schemas/plan.json`.

Schema paths above are relative to the installed kit copy root. Work and planning templates include their own named sections for repository facts, norms, and previous stage outputs. Read the full generated request and its references, and retain its `instruction_id` in the result. Follow `stages/README.md` for work submission and completion, `checks/README.md` for judgments, and `plans/README.md` for planning.
