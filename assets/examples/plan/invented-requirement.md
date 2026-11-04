## Approval policy

The fixed policy for this example is `[approval] plan = "requirements"` and `meta_loop_edit = "confirm"`.

## Fixed inputs

Original user request: Rename the save button to "Save" and preserve existing storage behavior.
Task supplied to start: Rename the save button to "Save".
Template: implement, with behavior and command:test checks and no stage gate. This is the first plan under the default approval policy.

## Target

Completion conditions: C1: The save button reads "Save". C2: Existing storage behavior is preserved. C3: Saving also requires entering a new mandatory account identifier.
Open questions: none. The planner decided C3 was required without asking the human.
Adopted stages: implement, preserving template checks and transitions. No deviations or external operations.
Verification: behavior checks C1 and C3 through the UI; command:test checks C2.
Plan approval: Obtain human approval before starting work.
