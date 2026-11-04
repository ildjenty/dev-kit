## Approval policy

The fixed policy for this example is `[approval] plan = "requirements"` and `meta_loop_edit = "confirm"`.

## Fixed inputs

Original user request: Rename the save button to "Save" and preserve existing storage behavior.
Task supplied to start: Rename the save button to "Save".
Template: implement, with behavior and command:test checks and no stage gate. This is the first plan under the default approval policy.

## Target

Completion conditions: C1: The save button reads "Save".
Open questions: none.
Adopted stages: implement, preserving template checks and transitions. No structural differences or external operations.
Verification: behavior checks the rendered button for C1; command:test also runs.
Plan approval: Obtain human approval before starting work.
The plan explicitly excludes storage compatibility because the task supplied to start did not mention it.
