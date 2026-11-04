These JSON Schema definitions specify plans, work reports, and judge and planner results. Follow the schema supplied in each request and retain its instruction ID. Paths such as `schemas/progress.json` are relative to the installed kit copy root.

- `plan-template.json`: Reusable loop template with stages, checks, gates, limits, and transitions.
- `plan.json`: Task-specific plan with completion condition IDs and check targets.
- `detection-result.json`: Detection test observations, including actual judgments and grounds.
- `progress.json`: The termination block. Requires the instruction ID and termination kind in a termination block. The kinds are `completion-claimed`, `confirmation-required`, and `work-stopped`.
- `judgment.json`: The judgment result. Used for a judge's final message and includes the instruction ID.
- `planner-result.json`: The planner result. Wraps the instruction ID and plan JSON. Devkit validates plan content against `plan.json`.
- `finding.json`: A finding. Contains a finding, an exemption claim and evidence, and references to resolved findings.
- `question.json`: A question. Contains the approval target, answer options, recommendation, stop reason, stages, and evidence. Decision approval options are `approve`, `reject`, and `defer`.
- `instruction.json`: An instruction. Contains the brief, extraction path, question, or external operation approval target according to the instruction kind.
- `decision.json`: A decision. Contains a decision, applicability conditions, supersession and dependencies, rejected alternatives, and evidence.
- `evidence.json`: Evidence. Contains investigation results, sources, and a verification method.
- `reference.json`: A reference. Defines a reference to a record within the work or an explicit external source.

Finding, Question, and Reference are defined once in `finding.json`, `question.json`, and `reference.json`. Other schemas use `$ref` to share these definitions. Their resource URIs are `urn:devkit:form:finding`, `urn:devkit:form:question`, and `urn:devkit:form:reference`.

When validating schemas, resolve `$ref` with the definitions in this directory, including `plan-template.json` for plans and `judgment.json` for detection results. Target and norm matching, record reference existence, judgment acceptance, and decision state are checked by devkit in addition to schema validation.

`progress.json` imposes no kind-specific required fields. Fields such as work performed may be omitted. Supplemental termination block fields are not rejected solely because their keys are unknown.
