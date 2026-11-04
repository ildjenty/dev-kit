## Approval policy

The fixed policy for this example is `[approval] plan = "requirements"` and `meta_loop_edit = "confirm"`.

## Fixed inputs

Original user request: Rename the save button to "Save", preserve existing storage behavior, and make the save status clearer.
Task supplied to start: Rename the save button to "Save".
Template: implement, with behavior and command:test checks and no stage gate. This is the first plan under the default approval policy.

## Target

Completion conditions: C1: The save button reads "Save". C2: Existing storage behavior is preserved. C3: Save status wording follows the human's answer to the open question before implementation.
Open questions: Which status wording and states should be displayed after saving? The request does not specify them; do not guess.
Adopted stages: implement, preserving template checks and transitions. No structural differences or external operations.
Verification: behavior checks the rendered button for C1 and the agreed status wording for C3; command:test checks C2.
Plan approval: Present the original request, start task, all three conditions and the open question. Obtain the human's clarification through rejection and replanning, and approve the revised plan before starting work.
