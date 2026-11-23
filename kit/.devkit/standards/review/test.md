# Test

Which executable evidence detects failures of the agreed behavior. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

No plan judge: delivery owns release-unit and test-plan coverage. This quality owns tests of implemented behavior.

## Code

Check conformance to each test row and all owned principles. For each assigned completion condition follow the method chosen in the plan. For `test`, verify that the named test exists, runs through a configured check, and its expectation follows from the completion condition text; inspect the test and execution path. Conditions assigned `screen_record` or `owner_check` are not judged by this test judge: the pull request body lists them for the owner. Never require converting another planned method into an automatic test. Verify user-visible contracts through their entry points, including CLI arguments, stdout, specified stderr and exit codes. Derive expectations from agreed contracts. Check selection, failure detection, preserved behavior, changed contracts and reproducibility. For rejection, checks or limits, verify a record that removing or breaking the processing makes the test fail. Identify tests that duplicate guarantees, test frameworks or libraries alone, or assert mocks or implementation details outside the contract. Verify behavior-preserving structural changes separately from requested behavior changes. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

## Blocking

For conditions assigned `test`, the named test does not exist or run through a configured check, its expectation does not follow from the condition, a relevant user-visible contract is untested, or it still passes when the target rejection or check is removed. A blocking condition of an owned principle applies within this assigned scope. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Conditions assigned `screen_record` or `owner_check`, and requests to replace those methods with automatic tests. Requests for tests duplicating the same guarantee under the same premises without detecting a different failure. Unnecessary tests are nonblocking candidates for removal.
