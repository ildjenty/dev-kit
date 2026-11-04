# Plan

Review a proposed plan for request coverage, gates, deviations, unresolved requirements, split work, size, inspections, and verification.

## Criteria

The plan template rules and approval rules in `plans/README.md` are part of the criteria, together with the approval policy in the fixed `devkit.toml` supplied for a plan judgment (or the policy stated within each detection example) and the schemas devkit supplies in the request (`schemas/plan-template.json`, `schemas/plan.json`, and `schemas/finding.json`).

## What to check

- **Request coverage**: Compare the fixed original user request, when it was observed, and the task supplied to start with the completion conditions and `open_questions`. Every requested requirement must be preserved: do not drop requirements, and do not invent requirements absent from the request. Ambiguity belongs in `open_questions`, not in a guess. When the original request was not observed, state that limitation and use the supplied task without treating it as the original.
- **Gates**: Check that reasons are given as the plan template requires (`schemas/plan-template.json` and `plans/README.md`), and that human confirmation follows `plans/README.md` and the approval policy in the fixed `devkit.toml` supplied for a plan judgment (or the policy stated within each detection example). Under the default policy, the first plan, a plan that changes completion condition IDs or wording from the last human-approved version, a plan with deviations, and a plan that declares external operations require human approval. Replanning with the same conditions and no deviations, external operations, or open questions does not require renewed approval. A nonempty `open_questions` array always requires human approval, regardless of policy.
- **Deviations**: Check deviations from the plan template, including omitted stages (`schemas/plan.json` and `plans/README.md`), and whether applicable findings are marked `needs-decision` (`schemas/finding.json`, a finding that requires a human decision).
- **Unresolved requirements**: Check that the planner records requirements it cannot decide in `open_questions` rather than filling them by assumption.
- **Split work**: Check whether unit scopes overlap and report a concrete collision scenario. Unit IDs are unique within the plan. Adding or removing split is a template deviation; stages already completed in this work and omitted by replanning are not removed-stage deviations.
- **Size**: Check whether stages can be reduced.
- **Inspections**: Check whether the inspections needed for the change type (UI, data, external communication, or structure) are included.
- **Verification**: Check that each completion condition has a way to verify it: a check run by a verification command, a judge, or a judgment command, or a human gate. Checks are not necessarily mechanical; conditions about agreeing on requirements are verified by judges and human gates.

## Blocking

The plan drops or invents requirements, bypasses required human approval, treats an unresolved requirement as agreed without a human decision, leaves a completion condition with no way to verify it, or gives units overlapping scopes that can cause conflicting changes.
