## Approval policy

The fixed policy for this example is `[approval] plan = "requirements"` and `meta_loop_edit = "confirm"`.

## Request

The template is purpose → implement. C1 requires the save button text to be "Save"; C2 requires preserving existing storage behavior. Only UI text changes; data, communication, and structure do not change.

## Target

# Plan

- Adopted: purpose → implement. Preserve the template's transitions, gates, and required checks.
- Checks in implement: behavior verifies rendering and interaction on the C1 target screen; command:test verifies existing storage behavior for C2.
- Plan approval: This is the first plan under the default approval policy. Start work without human approval because this is only a text correction.
- Stage gate: No additional gate is required by this template.
- Size: Confirm the request's purpose in purpose, then change the text in implement.
