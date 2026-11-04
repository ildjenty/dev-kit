## Approval policy

The fixed policy for this example is `[approval] plan = "requirements"` and `meta_loop_edit = "confirm"`.

## Request

The template is purpose → implement. C1 requires the save button text to be "Save"; C2 requires preserving existing storage behavior. Only UI text changes; data, communication, and structure do not change. A previous plan with both stages and these exact condition IDs and wording has already received human approval. A retry requires replanning without changing those conditions.

## Target

# Plan

- Adopted: implement. Purpose is unnecessary for this retry and is omitted within the configured loop rules. Show the removed purpose stage in the symmetric structural difference against the approved remaining plan. Preserve the required behavior and command:test checks. There are no declared external operations or open questions.
- Checks in implement: behavior verifies rendering and interaction on the C1 target screen; command:test verifies existing storage behavior for C2.
- Plan approval: Under the default approval policy, do not request renewed approval for this retry because completion conditions are unchanged and there are no external operations or open questions; the structural difference alone does not require approval.
- Stage gate: No additional gate is required by this template.
- Size: Retain only the necessary implement stage; do not add stages only to define return destinations.
