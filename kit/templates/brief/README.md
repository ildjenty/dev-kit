# Brief request templates

These files contain the work, judgment, and planning request templates.
The caller resolves their text from `.devkit/kit/<version>/templates/brief/`
and passes it in the corresponding input's `RequestTemplate` field.
brief applies that text with `form.ApplyTemplate` and performs no I/O.
engine writes the resulting request files.

- `work.md.tmpl`: `brief.WorkInput`, plus `StageJSON`, `PlanJSON`, `ChecksJSON`,
  `FindingsJSON`, `ProgressJSON`, and the `TerminationCompleted`,
  `TerminationQuestion`, and `TerminationStopped` values defined by form.
- `judgment.md.tmpl`: `brief.JudgmentInput`, plus `InstructionIDJSON`.
  check selects the request content.
- `planner.md.tmpl`: `brief.PlannerInput`, plus `TemplateJSON`, `CurrentJSON`,
  `RemainingJSON`, `ChecksJSON`, `FindingsJSON`, and `InstructionIDJSON`.

Each file is a complete `text/template` source. Work and planning templates
include their own named sections for repository facts, norms, and previous
stage outputs; the caller does not need to load separate partial templates.

Repository references: `docs/architecture/modules.md#brief`,
`docs/architecture/modules.md#kit`, and `docs/parts/brief.md`.
