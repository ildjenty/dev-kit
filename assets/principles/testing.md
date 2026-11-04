# Testing principles

Tests verify specifications. In repositories without a test mechanism, create one during the work and register the verification command that runs tests under `[commands.test]` in `devkit.toml`. Do not write tests that merely reproduce the implementation. If tests are hard to write, consider whether the design has a problem before working around it through test-writing techniques.

## 1. Choose test targets based on errors and detection methods

- **Applicability**: When deciding whether to write tests.
- **Exemptions**: When the target contract requires rejection or non-display for all inputs and states, with no legitimate allowance or display case, do not require adding a positive control. Show the contract's source and scope, evidence that the target processing ran, and a record that breaking rejection or non-display makes the same test fail for the intended reason.
- **Principle**: Choose test targets based on possible errors, their impact, and what other checks can detect. Even errors that raise exceptions cannot be detected unless other checks exercise that path. Do not exclude targets based solely on behavior when code is deleted or reachability from production; assess effects on product behavior, distributed artifacts, and reliability of verification results. Compare creation and maintenance burdens with error impact, recurrence likelihood, and the burden of alternative verification. Do not omit tests merely because they are hard to write before implementation, or judge value by the ratio of setup code length to assertion length. Behavioral verification used instead must detect the same failures. Include the following targets.
  - Security processing: Authorization, tenant isolation, fail-closed behavior, idempotency, transaction boundaries, and input validation at trust boundaries. Include declarations (RLS policies, GRANTs, constraints) and the DB or library guarantees they rely on.
  - Logic errors: Processing that leads not only to incorrect results but also to exceptions for particular inputs or states, data loss, or major feature outages.
  - Contracts with external specifications: Verify what external services actually return, such as headers and status codes, separately from your handling of those responses. Specification-based mocks or recorded responses may be used for your handling. Even with real services or emulators, state the contracts verified and differences not reproduced.
  - Completeness: Rules where omissions on addition cause errors, such as "All tables have RLS."
  - Positive controls: Minimal "allowed" or "displayed" tests that prevent tests for "rejected" or "not displayed" from succeeding without exercising the target processing because of mistakes in conditions or similar errors.
- **Violations and signs**:
  - Merely restating declarations or processing and checking only errors detectable by other checks.
  - Checking libraries, frameworks, or the DB itself without showing the relationship to guarantees the product depends on.
  - Excluding code generators or helpers affecting verification results because production does not call them.
  - Testing simple helpers separately without judging whether tests using them already verify them sufficiently.
  - No positive controls for "rejected" or "not displayed" tests that pass without executing the target processing.
  - Omitting automated tests solely because pre-implementation setup is difficult or setup code is long.
- **Before and after**: "Exclude quoted fields because they raise exceptions and will be noticed at runtime" → "Keep a test verifying loading because no other check exercises that input."
- **blocking**: No verification detects serious failures such as security errors, data loss, or major feature outages.
- **Evidence**: How possible failures correspond to detection methods, and the burdens of creation, maintenance, and alternative verification. For external contracts, the verified scope and differences substitutes cannot reproduce.

## 2. Consolidate tests that detect the same failures under the same assumptions

- **Applicability**: When deciding which layer should contain tests.
- **Exemptions**: Even when tests verify the same property, their execution paths, authority, configuration, or connections between layers differ. Consolidating them into a lower-cost layer would prevent detecting failures caused by those differences.
- **Principle**: Consolidate tests that detect the same failures under the same assumptions into lower-cost layers. Reduce duplication between type checks and runtime validation only when they guarantee the same properties over the same input range. Do not exclude tests for converting external input, stored data, or runtime configuration into validated types merely because those types exist.
- **Violations and signs**:
  - Keeping tests in multiple layers that detect only the same failures under the same assumptions.
  - Removing tests for different paths or authority because they verify the same property.
  - Removing boundary conversion or validation tests based on internal types.
  - So many snapshot tests that the specification is unclear.
- **Before and after**: "Confirmed that other tenants' rows cannot be read, so remove update tests" → "Keep both because reading and updating verify different paths and authority."
- **blocking**: None (record as reference information).
- **Evidence**: Differences in failures detected and assumptions for each test, and guarantees remaining after consolidation.

## 3. Derive expected values from completion criteria, not from the implementation

- **Applicability**: When writing expected values in tests.
- **Exemptions**: When recording current outputs confirmed as behavior to preserve for compatibility in characterization tests for an existing system without documented specifications. Deriving expected values from inferred specifications would miss behavior that must be preserved.
- **Principle**: Derive expected values from completion criteria (specifications), not by copying the implementation. Provide concrete tests for each completion criterion and make them runnable through registered verification commands. When code and tests are created from the same understanding, the same misunderstanding can make them agree and leave errors undetected.
- **Violations and signs**:
  - Copying function outputs directly into expected values.
  - Rewriting expected values to match implementation output to make tests pass.
- **Before and after**: Writing the equivalent of `expect(calc(input)).toEqual(calc(input))` → Derive `expect(calc({ qty: 3, unit: 100 })).toBe(300)` from completion criteria
- **blocking**: None (record as reference information).
- **Evidence**: The source of expected values (which statement in the completion criteria).

## 4. Verify that tests fail and that they keep passing

- **Applicability**: When verifying that tests actually check something.
- **Exemptions**: When no contract-preserving internal change is planned and internal restructuring would be needed solely to verify that tests keep passing. Do not make changes solely to verify continued passing, because restructuring just for verification creates unnecessary rework.
- **Principle**: Check whether breaking changes detect the intended property, and evaluate implementation dependence from the contract observed by the test.
  - Failing: Check whether behavior-breaking changes violate the property the test is intended to guarantee. Judge whether changes such as inverting a condition, removing a check, or returning a default instead of failure correspond to the failure reason.
  - Keeping passing: In work that actually changes internals, verify that contract-preserving changes pass tests. Follow principle 5 (Verify behavior without tracing the implementation) to judge implementation coupling by whether the observed subject is part of the contract.
- **Violations and signs**:
  - Tests for "zero results," "rejected," or "unchanged" also pass when target data does not exist.
  - Contract-preserving internal changes fail tests because of implementation differences outside the contract.
- **Before and after**: "The test passed, so rejection was verified" → "Verify that removing the rejection check makes the test fail for the intended reason, and in work changing internals, verify that tests for the same contract keep passing."
- **blocking**: In security-processing tests, the breaking change does not violate the intended guaranteed property, or the test fails for unrelated reasons.
- **Evidence**: How broken behavior corresponds to the failure reason, and the contract observed by tests. If internals changed, records of passing tests after contract-preserving changes.

## 5. Verify behavior without tracing the implementation

- **Applicability**: When deciding what to assert.
- **Exemptions**: None.
- **Principle**: Assert behavior observable as a contract, such as return values, stored results, external requests, and effects that must not occur. Check contracts at user-visible entry points; for a CLI, verify arguments, standard output, and exit codes. Verify counts and order too if they are part of the contract. Do not depend on internal helper calls or internal state outside the contract. Each test must explain one contract under a given set of conditions and assert together the results and side effects required for that contract. Do not mix unrelated conditions or contracts. To verify detection capability, target one property, break the verification target with a minimal diff, run once to confirm failure, then immediately revert and confirm restoration of the source to its pre-change state. Use the pair of the test file's repository-relative path and its framework identifier as the test ID, and record the failure-inducing diff linked to the execution result. Judge validity under principle 4.
- **Violations and signs**:
  - Asserting mock return values directly (checking the mock rather than the target).
  - Asserting counts or ordering of internal helper calls outside the contract.
  - Not checking contractually required transmission counts or side effects because return values are correct.
  - Checking unrelated conditions or contracts in the same test.
  - The test name does not reveal what it guarantees.
- **Before and after**: "Check only the return value for order rejection" → "For the same rejection operation, check the return value together with the fact that neither inventory nor payment changes."
- **blocking**: None (record as reference information).
- **Evidence**: Conditions and contract explained by the test, and how each assertion corresponds to results or side effects it checks.

## 6. Choose cases from reachable failure conditions

- **Applicability**: When deciding test inputs, initial states, operation sequences, or points at which to induce failure.
- **Exemptions**: None.
- **Principle**: Based on contracts and execution paths, choose reachable inputs, states, and histories that cause distinct failures. Validate invalid input at boundaries; internally, assume guaranteed types and states. For stateful processing, choose guarantee-affecting conditions from duplicate execution, different ordering, interruption, resumption, and concurrent execution. Test names must express "under what conditions, what happens," and setup, execution, and verification must be written separately.
- **Violations and signs**:
  - Bypassing type guarantees solely to pass out-of-contract values.
  - Verifying history-dependent processing only with single executions from initial state.
- **Before and after**: "Run each request from an empty DB" → "Check whether required properties hold when requests rerun or resume after interruption too."
- **blocking**: An important guarantee depends on inputs, state, or history, but conditions that could violate it are missing from verification.
- **Evidence**: Paths reaching the selected conditions and failures to detect under those conditions.

## 7. Control unrelated variability and reproduce failure conditions

- **Applicability**: When writing tests dependent on time, randomness, order, external communication, or a DB.
- **Exemptions**: When intermittent failures have been isolated to defects in an external environment you lack authority to change, do not require fixing that external cause. Record the target version, environment, execution results, evidence for isolation, and conditions for resuming. Mark verification that could not be confirmed as `NOT ASSESSED`. Do not treat a passing rerun alone as a fixed cause.
- **Principle**: Control variability unrelated to verification from time, randomness, execution order, external communication, or DB rows left by other tests. Preserve contention or passage of time that must be detected as reproducible conditions. When fixing or substituting dependencies, show guarantees preserved and properties no longer verifiable. Isolate intermittent failures into product defects, interference between tests, or external environment problems, and fix the cause. A passing rerun alone is not a fix; do not remove the conditions that trigger the target failure just to make tests pass.
- **Violations and signs**:
  - Results change because of unrelated time, randomness, order, external communication, or DB rows left by other tests.
  - Replacing the DB with a substitute that always executes serially in tests for contention.
  - Treating an intermittently failing test as fixed because it passed on rerun.
- **Before and after**: "Wait for real time to pass in a deadline test" → "Control time and reproduce conditions where results change before and after the deadline."
- **blocking**: None (record as reference information).
- **Evidence**: How variability is controlled, failure conditions preserved, and guarantees preserved and lost through substitution.

## 8. Distinguish behavior to preserve from contracts to satisfy after the change

- **Applicability**: When changing existing code.
- **Exemptions**: None.
- **Principle**: For behavior selected for testing under principle 1, verify contracts according to the purpose of the change. For behavior-preserving changes, record behavior to preserve in existing or characterization tests; if no mechanism exists, create and register one as described at the start of this document. Because characterization tests cannot demonstrate failure before implementation, verify their detection capability through the procedure in principle 5. For defect fixes or agreed specification changes, test the contract required after the change. Only if the old implementation violates the new contract, confirm that tests fail under conditions causing that violation. For changes where every outcome permitted by the old contract is also permitted by the new one, show guarantees waived and preserved, and verify test detection capability through the procedures in principles 4 and 5. Distinguish observed current behavior from the specification considered correct; do not include errors being fixed in behavior to preserve. After changing, verify that preserved behavior remains and changed behavior satisfies the new contract.
- **Violations and signs**:
  - Changing existing code without recording behavior to preserve.
  - Freezing errors being fixed as expected values that must continue to be met.
  - When the old implementation violates the new contract, not verifying that tests fail under conditions causing that violation.
  - Relaxing constraints without showing guarantees waived and preserved.
  - Relaxing constraints without verifying test detection capability through the procedures in principles 4 and 5.
- **Before and after**: "Use the current ability to read other tenants' rows as the expected value" → "Test the contract that they must not be readable, and verify failure before the fix and success afterward."
- **blocking**: A change affecting serious failures such as security processing lacks verification of behavior to preserve or to fix.
- **Evidence**: Contracts for preserved and changed behavior, tests checking each, and results before and after. For relaxed constraints, guarantees waived and preserved and records of verifying detection capability through the procedures in principles 4 and 5.

## 9. Fix test failures against contracts and evidence

- **Applicability**: When tests fail.
- **Exemptions**: None.
- **Principle**: Classify failure reasons as implementation errors, test errors, or changed assumptions. Fix implementation errors in the implementation; do not weaken agreed guarantees or adjust expected values to follow implementation output merely to eliminate failures. For agreed specification changes, test errors evident from specifications, or fixes preserving guarantees, update tests with supporting evidence. When specifications are unclear or contradictory, or guarantees need weakening, seek human judgment only for unagreed changes in meaning.
- **Violations and signs**:
  - Adjusting a failing test's expected values to match current output.
  - Weakening agreed guarantees by skipping tests or deleting assertions.
  - Seeking human judgment again for agreed specification changes or corrections of obvious typos.
- **Before and after**: "Adjust expected values to match output" → "Update expected values based on the agreed change to tax-inclusive prices. Seek judgment if tax handling has not been agreed."
- **blocking**: An unagreed change in meaning changes expected behavior or agreed guarantees.
- **Evidence**: Classification of failure reasons, specifications or agreements supporting updates, and any unagreed decisions.
