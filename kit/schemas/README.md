This directory contains JSON Schema definitions for plans and fixed formats.
Plan validation belongs to internal/plan; applying supplied schemas belongs to internal/form.
For placement, see `docs/architecture/modules.md#kit` from the repository root. For responsibilities, see `docs/architecture/modules.md#plan` and `docs/architecture/modules.md#form`.

- `progress.json`: `form.Progress`. Requires the instruction ID and termination kind in a termination block. The kinds are `completion-claimed`, `confirmation-required`, and `work-stopped`.
- `judgment.json`: `form.JudgmentResult`. Used for a judge's final message and includes the instruction ID.
- `planner-result.json`: `form.PlannerResult`. Wraps the instruction ID and plan JSON. The plan package validates the plan content.
- `finding.json`: `form.Finding`. Contains a finding, an exemption claim and evidence, and references to resolved findings.
- `question.json`: `form.Question`. Contains the approval target, answer options, recommendation, stop reason, stages, and evidence. Decision approval options are `approve`, `reject`, and `defer`.
- `instruction.json`: `form.Instruction`. Contains the brief, extraction path, question, or external operation approval target according to the instruction kind.
- `decision.json`: `form.Decision`. Contains a decision, applicability conditions, supersession and dependencies, rejected alternatives, and evidence.
- `evidence.json`: `form.Evidence`. Contains investigation results, sources, and a verification method.
- `reference.json`: `form.Reference`. Defines a reference to a record within the work or an explicit external source.

Finding, Question, and Reference are defined once in `finding.json`, `question.json`, and `reference.json`. Other schemas use `$ref` to share these definitions. Their resource URIs are `urn:devkit:form:finding`, `urn:devkit:form:question`, and `urn:devkit:form:reference`.

Pass the JSON loaded by kit to `form.CompileSchemaWithResources`, with referenced schemas in a `map[string][]byte` keyed by resource URI. `judgment.json` needs Finding and Reference; `instruction.json` needs Question and Reference; `finding.json`, `question.json`, `decision.json`, and `evidence.json` need Reference. Schemas with no external references can use `form.CompileSchema`. Apply the result with `Schema.Validate` or `Schema.Decode`. The form package does not read files. The caller resolves target matching, record reference existence, judgment acceptance, and decision state.

The source of truth does not list additional required fields for each termination kind, so `progress.json` imposes no kind-specific requirements. Fields such as work performed may be omitted. Supplemental termination block fields are not rejected solely because their keys are unknown.
