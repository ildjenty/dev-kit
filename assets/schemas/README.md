These binary-owned JSON Schema definitions specify plans, work reports, repository structure records, notes, and judge and planner results. Follow the schema supplied as text in each request and retain its instruction ID. Each supplied schema bundles all referenced definitions so it can be used without retrieving other schema files. Work briefs also supply the supported `devkit note` kinds and their bundled schemas, and `schemas/structure.json` for structure records. References such as `schemas/progress.json` identify those definitions.

- `schemas/plan-template.json`: Reference loop pattern with stage instances and definitions, configured check names, gates, limits, and transitions.
- `schemas/plan.json`: Task-specific plan with a pattern name and content hash, instance IDs and definition IDs, completion condition IDs, check targets, and an optional `open_questions` array of unresolved requirements. Verification coverage is assessed by the plan judge and reviewed by the human; checks or human gates may verify conditions.
- `schemas/structure.json`: Repository structure with `modules` (`id`, `name`, `responsibility`, `path`) and directed `dependencies` (`from`, `to`). The version used for judgments and display is the content hash supplied by devkit, not an author-selected field. Module IDs must be unique and dependency endpoints must refer to recorded modules.
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

When validating schemas, resolve `$ref` with the bundled definitions, including `schemas/plan-template.json` for plans, and `schemas/plan.json` for planner results. Target and norm matching, record reference existence, judgment acceptance, and decision state are checked by devkit in addition to schema validation.

`schemas/progress.json` imposes no kind-specific required fields. Fields such as work performed may be omitted. Supplemental termination block fields are not rejected solely because their keys are unknown.
