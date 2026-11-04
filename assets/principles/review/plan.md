# Plan

Review a proposed plan for request coverage, gates, structural differences, unresolved requirements, split work, size, inspections, and verification.

## Criteria

The loop rules and reference pattern guidance and approval rules in `plans/README.md` are part of the criteria, together with the approval policy in the fixed `devkit.toml` supplied for a plan judgment and the schemas devkit supplies in the request (`schemas/plan-template.json`, `schemas/plan.json`, and `schemas/finding.json`).

## What to check

- **Request coverage**: Compare the fixed original user request, when it was observed, and the task supplied to start with the completion conditions and `open_questions`. Every requested requirement must be preserved: do not drop requirements, and do not invent requirements absent from the request. Ambiguity belongs in `open_questions`, not in a guess. When the original request was not observed, state that limitation and use the supplied task without treating it as the original.
- **Gates**: Check that reasons are given as the plan template requires (`schemas/plan-template.json` and `plans/README.md`), and that human confirmation follows `plans/README.md` and the approval policy in the fixed `devkit.toml` supplied for a plan judgment. Under requirements (the default policy), the first plan, changed completion condition IDs or wording, declared external operations, and open questions require human approval. Pattern differences alone do not require approval. Replanning with the same conditions and no external operations or open questions does not require renewed approval. Always requires approval for every version. Deviation also compares structure against the last human-approved remaining plan, excluding adopted completed instances. A nonempty `open_questions` array always requires human approval, regardless of policy.
- **Structural differences**: Review the symmetric structural difference displayed against the last approved remaining plan, or the cited pattern for the first plan. Patterns are references, not constraints. Omitted stages, assignees, gates, checks, split units, and transitions may differ within the configured loop rules. Do not report a difference as a blocking finding solely because it differs from a pattern. Use `needs-decision` only for a concrete unresolved human decision.
- **Unresolved requirements**: Check that the planner records requirements it cannot decide in `open_questions` rather than filling them by assumption.
- **Split work**: Check whether unit scopes overlap and report a concrete collision scenario. Unit IDs are unique within the plan. Adding or removing split appears in the structural difference; it does not itself require approval under requirements. Already adopted completed instances are excluded from the remaining-plan comparison.
- **Size**: Check whether stages can be reduced.
- **Inspections**: Check whether the inspections needed for the change type (UI, data, external communication, or structure) are included.
- **Verification**: Check that each completion condition has a way to verify it: a check run by a verification command, a judge, or a judgment command, or a human gate. Checks are not necessarily mechanical; conditions about agreeing on requirements are verified by judges and human gates.

## Blocking

The plan drops or invents requirements, bypasses required human approval, treats an unresolved requirement as agreed without a human decision, leaves a completion condition with no way to verify it, or gives units overlapping scopes that can cause conflicting changes.
