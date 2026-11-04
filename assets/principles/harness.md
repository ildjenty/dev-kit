# Principles for the loop mechanism

## 1. Compare the target against agreed conditions independently of its author's explanation

- **Applicability**: When running a check. When judging at a plan review gate. When accepting work delegated through a brief.
- **Exemptions**: For a previous independent judgment, if the target version, agreed conditions, selected criteria and norm versions, verification scope, environment, and input can be confirmed to match the current case, and there is no new evidence overturning the judgment, omit only re-comparison within the matching scope. Show in the artifact the items confirmed to match and a reference to the reused judgment record.
- **Principle**: Independently compare the target's actual contents against the purpose, completion criteria, and constraints agreed in the brief or loop plan, without adopting the author's reasoning or self-assessment. Treat the author's explanation as a hypothesis to verify. Use `NOT ASSESSED` if necessary judgment criteria cannot be confirmed.
- **Violations and signs**:
  - Using the author's explanation as the basis for judgment without checking it against the target and agreed conditions.
  - Returning `PASS` or `FAIL` without being able to confirm necessary judgment criteria.
  - A check keeps returning `PASS` even when the target changes, or later stages and humans keep finding problems it should have caught.
  - Blocking a target that qualifies for a principle's exemption and shows why the requirement was waived, in the same way as a violation without such reasons.
- **Before and after**: "Judged after being convinced by the author's explanation" → "Observed the target's contents, checked how they correspond to the agreed conditions in the brief or loop plan, and judged."
- **blocking**: The judgment depends on the author's unverified explanation.
- **Evidence**: How observations from the target correspond to the agreed purpose, completion criteria, and constraints.

## 2. Define which stage to return to when assumptions fail

- **Applicability**: When creating a loop plan. When the router decides to "return to a previous stage." When assembling multiple stages.
- **Exemptions**: When a minimal plan covers only what changes, does not depend on outputs from other stages, and adding a stage solely to define a return destination would create unnecessary planning.
- **Principle**: In multistage plans, show which stage's output provides each stage's assumptions, and define where to return when an assumption fails. Follow principle 4 of `principles/working.md` to investigate causes and shared assumptions when failures continue. If an upstream output is confirmed unusable as an assumption, return to the stage that decided it and revisit dependent downstream decisions. If the assumption can be maintained and the error is confirmed to be in the current stage, fix it there. Follow principle 1 of `principles/designing.md` for identifying hard-to-change decisions and when to finalize them.
- **Violations and signs**:
  - Even after confirming an assumption has failed, continuing downstream fixes without identifying the stage that decided the assumption.
  - Returning upstream because of a current-stage error despite confirming that upstream assumptions can be maintained.
- **Before and after**: "Kept compensating for errors in the structure assumed by implementation through implementation fixes" → "Confirmed that the structure output could not be used as an assumption, returned to the stage that decided that output, and revisited dependent downstream decisions."
- **blocking**: Repeatedly fixing the same stage under the same assumption even though that assumption has failed.
- **Evidence**: Observations checking whether assumptions can be maintained, how failed assumptions correspond to the outputs and stages they depend on, and reasons for choosing the return destination and downstream decisions to revisit. When fixing in the current stage, why its assumptions can be maintained and the error belongs to that stage.

## 3. When rule violations recur, choose enforcement suited to the required guarantees

- **Applicability**: During retrospectives, when deciding where to incorporate findings or failures, or when reviewing rules and enforcement mechanisms.
- **Exemptions**: When loss is ongoing and waiting to confirm causes or compare enforcement mechanisms would increase damage, defer cause confirmation and comparison only for executing known containment procedures. This does not apply to choosing permanent measures. Show in the artifact observations establishing the ongoing loss and why waiting would worsen it, containment procedures and completion criteria, conditions for resuming omitted cause confirmation and comparison, and who receives the handoff.
- **Principle**: In retrospectives, consider moving items in `rules.md` whose enforcement is not implemented and repeatedly found violations into machine checks (tests, lint, types, pre-commit, or validation commands). When rule violations recur, verify the rule's validity and the causes of violation. Choose enforcement by separately considering whether it detects the necessary property, runs on the required paths, and can stop processing before loss occurs. Do not repeatedly address violations that written instructions failed to prevent solely by adding more text. Define where the problem should have been caught; by default, consider incorporating lessons into machine checks, templates, review documents, principles, and skills, in that order. Follow `[lessons] destinations` in `devkit.toml` for priority. Incorporate lessons into the repository's norms. If not incorporating a lesson, state why.
- **Violations and signs**:
  - Addressing a violation only by adding text even though the same violation has been recorded again.
  - Assuming enforcement is stronger from its name alone without checking detection scope, execution paths, or when it can stop processing.
- **Before and after**: "The same violation recurred, so added a warning" → "Checked the rule and cause, identified conditions that can be detected, paths that must pass through checks, and the point where processing can stop before loss, then chose enforcement."
- **blocking**: None (record as reference information).
- **Evidence**: Verification results for the rule's validity and causes of violation, detection scope, execution paths, and prevention timing before and after the change, and how the guarantees change as a result.

## 4. Choose mechanisms based on the failures they prevent and their operational burden

- **Applicability**: When deciding to add checks, hooks, or procedures, make them required, expand their applicability, or organize and reduce them.
- **Exemptions**: When an external contract specifies the mechanism and scope, comparison cannot change them, and comparison would only delay fulfilling the obligation.
- **Principle**: Choose mechanisms and scope by comparing the impact and recurrence likelihood of failures to prevent against the burden of execution, maintenance, and handling false positives. On recurrence too, verify the rule's validity and causes, and compare options including correcting, replacing, or removing existing mechanisms. Choose enforcement from detection scope, execution paths, and prevention timing, and compare the burdens of alternatives that preserve the required guarantees.
- **Violations and signs**:
  - Making a mechanism required for all work based only on having detected a violation or on recurrence count.
  - Not comparing lower-burden alternatives that preserve the required guarantees.
- **Before and after**: "Made a useful check required for all changes" → "Compared failure impact and operational burden, required it for high-impact changes, and limited its scope for minor changes to where existing verification meets the required guarantees."
- **blocking**: A concrete alternative preserves the required guarantees and reduces burden, but there is no reason for choosing a more burdensome required mechanism.
- **Evidence**: Impact and recurrence likelihood of failures to prevent, guarantees preserved and risks left by each mechanism, execution, maintenance, and false-positive handling burdens, and reasons for the chosen scope.
