# Working approach

## 1. Restate the goal and problem in your own words before starting

- **Applicability**: When receiving a brief. When receiving a request.
- **Exemptions**: None.
- **Principle**: Before starting, restate the goal, completion criteria, and problem in your own words. Verify hypotheses about causes and methods whose choice has been delegated as candidates. Preserve explicit constraints and specified methods; if they need changing, show reasons and effects and seek confirmation. If the restatement differs from the request, confirm before starting. On completion, state what was rejected, what was omitted, which inputs were not consulted, and what you decided yourself.
- **Violations and signs**:
  - Starting by repeating the request verbatim.
  - Adopting methods whose choice was delegated without verifying the purpose.
  - Treating methods specified as constraints as candidates and starting unnecessary comparisons.
  - Reporting only completion without returning rejected options or omitted procedures.
- **Before and after**: "The cache is said to be the cause, so rebuild it" → "The purpose is to eliminate delays in displaying the list. Preserve the constraint against adding dependencies, and measure whether the cache is the cause. Completion means meeting the agreed display time."
- **blocking**: None (record as reference information).
- **Evidence**: The restatement and how it corresponds to the request's constraints and completion criteria. The list explicitly stated on completion.

## 2. Divide work into verifiable and reversible units, and do not overlap dependent or interfering changes

- **Applicability**: When dividing work. When deciding whether to proceed to the next task or run tasks concurrently.
- **Exemptions**: Only when agreed requirements or external contracts require irreversible operations, or operations whose reversal would violate agreed safety requirements, and valid approval or delegation covers that irreversibility, waive only the requirement to restore those operations to their original state. Do not waive verification of each work unit or avoidance of dependence on unverified results and interference with each other's verification. Show in the artifact the operations, why restoration is impossible or reversal would worsen matters, approval or delegation scope, how execution results will be verified, and acceptable states and responses on interruption or failure.
- **Principle**: Divide work into units whose results can be verified and reverted if there is a problem. Do not overlap work dependent on unverified results or changes that interfere with each other's verification.
- **Violations and signs**:
  - Proceeding with downstream work using unverified output as an established assumption.
  - Another task rewrites a target under verification, making it unclear which change was verified.
  - No decision on who will integrate and verify concurrent work or how.
- **Before and after**: "Rewrote an implementation under verification while building downstream processing based on its output" → "Hold the target fixed, verify it, and check results before proceeding downstream. Investigate independent specifications concurrently, recording dependencies and integration ownership and procedures."
- **blocking**: None (record as reference information).
- **Evidence**: Work divisions and each unit's verification and reversal methods. Independence and dependencies of concurrent work, integration ownership and procedures, and verification methods.

## 3. Distinguish behavior changes from structural changes and verify each result

- **Applicability**: When changing behavior. When organizing structure. When structural changes are needed to prepare verification.
- **Exemptions**: None.
- **Principle**: Distinguish behavior-changing changes from behavior-preserving structural changes so each result can be verified. For each change, clarify what changes and what is preserved, and make each change's effects distinguishable from verification results.
- **Violations and signs**:
  - Mixing fixes and cleanup makes the cause of behavior changes indistinguishable.
  - Assuming cleanup has no effect without checking behavior preserved through structural changes.
- **Before and after**: "Fixed a defect while changing structure, then checked everything together at the end" → "To control reproduction conditions, extracted the calculation while preserving inputs and outputs, and verified that preservation. Then made the fix and separately verified its effect on the defect."
- **blocking**: None (record as reference information).
- **Evidence**: What each change preserves and verification results. If cleanup comes first, why it is necessary and records distinguishing the fix's effects from the cleanup's effects.

## 4. Revisit assumptions when fixes based on them repeatedly fail

- **Applicability**: When a fix fails a check.
- **Exemptions**: None.
- **Principle**: If fixes based on the same assumptions repeatedly fail the same checks, do not continue fixing under those assumptions. Write down the assumptions and follow principle 3 of `principles/verification.md` for hypothesis testing. During progressive refinement, follow principle 2 of `principles/harness.md` to decide whether to return to the stage that decided the assumption or fix it in the current stage.
- **Violations and signs**:
  - Repeated fixes make small changes to the same place.
  - Fixes begin with "This time for sure."
- **Before and after**: "Another fix: increase the timeout further" → "Question the assumption 'The network is slow.' Measurement showed a DB wait."
- **blocking**: None (record as reference information).
- **Evidence**: Written assumptions and observations used to test them. If returning to a stage, that the assumption was decided there.

## 5. Turn routine processing into commands and call them from procedures

- **Applicability**: When writing procedures in skills or playbooks. When noticing repeated execution or copying of the same procedure.
- **Exemptions**: For candidates found during other work, defer creation only if it is unnecessary for that work to succeed, record the candidate in the artifact, and decide adoption in a separate issue. Creation may also be deferred when existing tools cannot meet the need and creation and maintenance burdens exceed the burden of repeated work or errors from reconstructing the procedure; show in the artifact the candidate, what existing tools cannot meet, and the burdens compared. Waive only creation, not investigation and use of existing tools, or testing and caller migration if a tool is created. The principle's other requirements and blocking conditions apply.
- **Principle**: Turn routine processing with definable inputs, outputs, side effects, and failure results into commands (tools), and call them from skills and playbooks. Leave applicability conditions, invocation order, decisions, result verification, and records of execution and omission in procedures; do not copy command implementations into them. When noticing repetition or copying of the same procedure, consider consolidation, comparing procedure stability, expected future use, reconstruction errors, and creation and maintenance burdens. Before creating a tool, check for existing tools doing the same processing, add tests, and migrate callers being replaced.
- **Violations and signs**:
  - Multiple skills or playbooks duplicate the same processing implementation (command sequences). Calls to the same existing command do not count.
  - Adding a separate implementation for processing covered by existing tools without showing a difference in purpose.
  - Creating a shared command while leaving old copies in skills or playbooks.
- **Before and after**: "Three skills described record validation with the same three commands" → "Combined them into one command, replaced the three skills' procedures with calls to it, and listed it in the index."
- **blocking**: Adding an equivalent separate implementation for processing covered by existing tools without showing a difference in purpose in the artifact.
- **Evidence**: Tool and test results, access from the index, and replaced callers. If creation is deferred, the candidate record and reasons based on comparison of procedure stability, expected use, and burdens.

## 6. Proceed with execution decisions within agreement and delegation, and seek necessary approval

- **Applicability**: When tempted to ask "May I do this?"
- **Exemptions**: None.
- **Principle**: Proceed without repeated confirmation on execution decisions within the agreed purpose, scope, constraints, and delegated authority, and show the results. Stop when new approval or product decisions are needed, such as matters covered by the router's "human confirmation" (irreversibility, security, billing, external writes, disagreement with design documents, changes to conventions or the harness, or unagreed purpose). Respect confirmation conditions specified by people. Follow principle 9 of `principles/testing.md` for rewriting expected values or assertions. Judge from rules' purposes as well as their wording. A delegate must not decide matters requiring human judgment or proceed with provisional decisions without valid approval or delegation.
- **Violations and signs**:
  - Repeatedly confirming file creation or test additions within agreement and delegation.
  - Seeking approval again for approved operations whose target, environment, and scope have not changed.
  - Ignoring human-specified confirmation conditions because an operation is reversible.
  - Changing security settings or posting externally without valid approval or delegation.
  - A delegate proceeds on unapproved provisional decisions for matters requiring human approval.
- **Before and after**: "Deployment to this environment is approved, but ask again because it is an external write" → "Confirm that the approval's target, environment, and scope are unchanged, execute according to specified confirmation conditions, and show results." Stop and seek confirmation for unapproved payment-setting changes.
- **blocking**: Decisions or operations requiring human approval are executed without valid approval or delegation (including execution by delegates or as provisional decisions).
- **Evidence**: How decisions and operations correspond to agreed purpose, scope, constraints, and approval or delegation scope. The target, environment, and scope of existing approval, and specified confirmation conditions. If stopping, why new approval or product judgment is necessary.

## 7. Do not make unrequested changes

- **Applicability**: When deciding whether to include changes outside the request's scope in a diff.
- **Exemptions**: None.
- **Principle**: Outside the requested scope, neither change intent nor effects have been confirmed. Include fixes within the changed module that are inseparable from the requested change in the same diff, within the agreed scope, explaining their necessity. Do not implement improvements outside the module or unnecessary for the current change; report them as proposals. If independently deciding unspecified behavior (limits, defaults, or failure behavior), report that it was your judgment.
- **Violations and signs**:
  - Unnecessary renaming, formatting, or refactoring is mixed into the same diff as the actual change.
  - Logs or comments added for investigation remain.
  - Independently deciding limits, defaults, or failure behavior without reporting it.
- **Before and after**: "Also sorted imports while here" → "Propose import ordering as a separate change."
- **blocking**: Behavior-changing changes exist outside the request's scope.
- **Evidence**: Show which part of the request each file in the diff serves.
