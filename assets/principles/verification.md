# Fact verification principles

## 1. Do not present unverified matters as verified

- **Applicability**: When making statements about tools, APIs, versions, behavior, or causes.
- **Exemptions**: None.
- **Principle**: Explicitly mark uncertain implementations, guesses about external specifications, and unverified assumptions as such; do not state them as established facts. Verify facts using primary information. Judge whether information is primary by whether it directly observes or defines the claim and matches the target version and conditions, not by its medium. Official documentation alone does not guarantee correctness, and a blog or Q&A site alone is not grounds for exclusion. Record verification results with the versions, environments, inputs, and references under which they hold, and reuse them. Do not treat unrecorded investigation results or memory alone as reliable assumptions. An event observed once may be treated as a fact of occurrence, but does not alone establish frequency, reproducibility, cause, or general behavior; separately describe the scope that can be generalized. When documentation and actual behavior differ, record guaranteed contracts separately from observed behavior; do not replace contracts with observations alone. If relying on a difference, state the conditions and risks. After issuing a judgment, identify conditions that could overturn it and perform additional checks according to missing evidence.
- **Violations and signs**:
  - Implementing unverified external API specifications as established facts (writing nonexistent methods or parameters).
  - Deciding tool or dependency versions from memory or local environment versions.
  - Treating "it probably behaves this way" as established and proceeding on that assumption.
  - "Fixing" a guessed error cause and declaring resolution without checking the effect.
  - Unmarked provisional implementations or stubs are indistinguishable from real implementations.
  - Asserting frequency, reproducibility, cause, or general behavior from one observation.
  - Describing more than a mechanism actually verifies (describing a location-only check as "checking contents").
  - Using what is done elsewhere as evidence of correctness.
  - Not considering counterevidence after issuing a judgment.
  - Treating only a memory of "having investigated before" as a reliable assumption.
  - Accepting information solely because it is official documentation, and excluding it solely because it is on a blog or Q&A site.
  - Replacing documented guaranteed contracts with observed behavior.
- **Before and after**: "This library retries automatically" → "The documentation for the target version (URL) did not establish a retry guarantee. In the recorded environment and input, failure raised an exception without a retry. Behavior under other conditions is unverified."
- **blocking**: The diff includes implementations that treat unverified assumptions as established facts without explicitly marking them as hypotheses for verification.
- **Evidence**: References to primary information directly corresponding to claims (files and lines, URLs and relevant sentences, experimental procedures and results), and correspondence to the target versions, environments, and inputs. When documentation and actual behavior differ, records separating guaranteed contracts from observed behavior.

## 2. Write observations and inferences separately

- **Applicability**: When writing causes or reasons.
- **Exemptions**: None.
- **Principle**: Describe observations separately from inferences drawn from them. For inferences used in decisions, show which evidence led to the conclusion, the reasoning, and remaining alternative explanations. Do not count materials derived from the same observation as independent support. Code alone does not establish its implementation intent. A hypothesis in a question is a candidate, not a conclusion.
- **Violations and signs**:
  - Writing "It is not in the logs, so X is the cause" as a verified fact.
  - Not showing reasoning from evidence to conclusions or remaining alternative explanations.
  - Counting a log and a graph derived from it as independent support.
- **Before and after**: "The cache was the cause" → "The problem stopped reproducing when the cache was disabled. Because cache use changed, the cache may be involved. However, disabling it may also have slowed processing and reduced concurrency."
- **blocking**: None (record as reference information).
- **Evidence**: Reasoning from evidence to conclusions and remaining alternative explanations. Materials derived from the same observation are not counted as independent support.

## 3. Verify defect causes through observation

- **Applicability**: When investigating defect causes.
- **Exemptions**: None.
- **Principle**: Describe the occurring phenomenon precisely. Read all error messages, record when and under what conditions it occurs, and create reproduction steps. Distinguish changes for root-cause fixes, temporary responses, and hypothesis testing. Form hypotheses as "If this hypothesis is correct, this result should be observed," list alternative explanations predicting the same result, and choose observations or interventions that distinguish them. If a change also alters load, execution order, inputs, or other factors, distinguish their effects. If repeated hypothesis testing does not reveal the cause, stop guessing and increase observation. Remove conditions individually to identify the minimum conditions that cause the problem. Limit claims of established causes or root-cause fixes to the evidence obtained. If fixes based on the same assumptions keep failing the same checks, revisit assumptions instead of adding more fixes. Evaluate fix effectiveness against previous occurrence conditions and the causal relationships the fix changes, using a method that distinguishes unfixed and fixed states. Do not judge resolution solely from failure to reproduce; leave unverifiable scope marked as unverified.
- **Violations and signs**:
  - Continuing to write fixes under the same assumptions.
  - Treating temporary responses or hypothesis-testing changes as root-cause fixes.
  - Not distinguishing alternative explanations predicting the same result through observation or intervention.
  - Using a verification method that would succeed even before the fix, and relying only on non-reproduction as evidence of resolution.
- **Before and after**: "Increasing the timeout fixed it" → "This is a temporary measure to reduce order-processing timeouts. It affects order-processing wait time. Compare completion status and wait time before and after under previous occurrence conditions to verify its effect. The cause remains unconfirmed, so this is not considered a root-cause fix."
- **blocking**: A change with unverified cause or effect is treated as a root-cause fix, or temporary responses or experiments are performed without stating their purpose, affected scope, and method for verifying effects.
- **Evidence**: Reproduction steps and occurrence conditions; the change's purpose, affected scope, and effect-verification method; observations supporting the chosen explanation over alternatives; and verification results distinguishing before and after, with unverified scope.

## 4. Separate evaluation of measurements from explanations of the causes of differences

- **Applicability**: When evaluating measurements such as performance, speed, or counts, or explaining causes of differences or potential for improvement.
- **Exemptions**: None.
- **Principle**: Separate evaluation of measurements from explanations of the causes of differences, and answer the following questions according to the claim.
  1. Can the cause of the difference or potential for improvement be explained? Show observations or interventions that distinguish that explanation from alternatives. Differences in counts and similar quantities may be explained by correspondence between inputs and results. When explaining processing time or throughput, identify limiting factors (limiters) from profiles or counters. Guesses from reading code alone do not establish causes.
  2. Could different conditions change the conclusion? Judge whether measurement conditions represent the scope of the stated conclusion. Generalization to all of production requires production representativeness; state the limited scope of results from restricted inputs or environments.
  3. Can an upper-bound model of improvement be applied? Treat the bound as a model with assumptions, and check serial and parallel relationships among processing and the effects of changes on the costs of other processing. Do not immediately reject results exceeding predictions; investigate measurement errors and errors in model assumptions.
  4. Is processing failing? Check failure counts and output correctness (rejection is fast; timeouts are slow).
  5. Can the difference be considered an effect? Judge whether variability can explain the observed difference.
  6. Is it meaningful? Its proportion within the whole.
  7. Do execution records support the claim? Judge whether the measured processing is the target whose performance is being described.
  Judge whether the measured target, conditions, correctness, and variability support the claim. Mark scope lacking necessary evidence as "cannot determine." Use exploratory measurements to understand variability and candidates, then verify hypotheses and judgment criteria developed there through additional measurements confirming effects. Do not establish effects from exploratory results alone.
- **Violations and signs**:
  - Establishing effects from hypotheses or judgment criteria developed from exploratory results without additional measurements.
  - Using results under limited conditions for performance claims beyond that scope.
  - Asserting causes of differences or potential for improvement without observations or interventions distinguishing alternative explanations.
  - Asserting causes of processing time or throughput without observing limiting factors.
  - Dismissing measurements exceeding predictions without checking assumptions of the improvement upper-bound model.
- **Before and after**: "It became faster" → "For the recorded inputs and environment, output correctness was verified and processing time decreased even after accounting for variability. Hypotheses and judgment criteria from exploration were confirmed through additional measurements. Applicability to all of production and the cause of the difference remain unverified."
- **blocking**: The measured target, conditions, correctness, or variability do not support the claim; explanations of causes of differences or potential for improvement lack observations or interventions distinguishing alternatives; or observations of limiting factors necessary to explain processing time or throughput are missing.
- **Evidence**: Answers to the questions relevant to the claim and records of measured targets, conditions, correctness, and variability. For explanations of causes of differences or potential for improvement, observations or interventions distinguishing alternatives; for explanations of processing time or throughput, observations of limiting factors; when using an improvement upper bound, reasons the model applies. A distinction between exploratory measurements and measurements confirming effects.
