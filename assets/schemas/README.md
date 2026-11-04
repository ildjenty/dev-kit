These binary-owned JSON Schema definitions specify plans, work reports, and judge and planner results. Follow the schema supplied as text in each request and retain its instruction ID. References such as `schemas/progress.json` identify those definitions.

- `schemas/plan-template.json`: Reusable loop template with stages, checks, gates, limits, and transitions.
- `schemas/plan.json`: Task-specific plan with completion condition IDs, check targets, and an optional `open_questions` array of unresolved requirements. Every completion condition must be covered by a required check; devkit validates that coverage in addition to the schema.
- `schemas/detection-result.json`: Detection test observations, including actual judgments and grounds. The judge receives example bodies without case kinds or expected judgments; devkit compares the results with expectations.
- `schemas/progress.json`: The termination block. Requires the instruction ID and termination kind in a termination block. The kinds are `completion-claimed`, `confirmation-required`, and `work-stopped`.
- `schemas/judgment.json`: The judgment result. Used for a judge's final message and includes the instruction ID.
- `schemas/planner-result.json`: The planner result. Wraps the instruction ID and plan JSON, referring to `schemas/plan.json` for plan validation.
- `schemas/finding.json`: A finding. Contains a finding, an exemption claim and evidence, and references to resolved findings.
- `schemas/question.json`: A question. Contains the approval target, answer options, recommendation, stop reason, stages, and evidence. Decision approval options are `approve`, `reject`, and `defer`.
- `schemas/instruction.json`: An instruction. Contains the brief, extraction path, question, or external operation approval target according to the instruction kind.
- `schemas/decision.json`: A decision. Contains a decision, applicability conditions, supersession and dependencies, rejected alternatives, and evidence.
- `schemas/evidence.json`: Evidence. Contains investigation results, sources, and a verification method.
- `schemas/reference.json`: A reference. Defines a reference to a record within the work or an explicit external source.

Finding, Question, and Reference are defined once in `schemas/finding.json`, `schemas/question.json`, and `schemas/reference.json`. Other schemas use `$ref` to share these definitions. Their resource URIs are `urn:devkit:form:finding`, `urn:devkit:form:question`, and `urn:devkit:form:reference`.

When validating schemas, resolve `$ref` with the bundled definitions, including `schemas/plan-template.json` for plans, `schemas/plan.json` for planner results, and `schemas/judgment.json` for detection results. Target and norm matching, completion condition coverage, record reference existence, judgment acceptance, and decision state are checked by devkit in addition to schema validation.

`schemas/progress.json` imposes no kind-specific required fields. Fields such as work performed may be omitted. Supplemental termination block fields are not rejected solely because their keys are unknown.
