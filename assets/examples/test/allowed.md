## Request

Review tests for an internal reorganization of an undocumented legacy amount CLI. Approved C1: Preserve `amount 3 100` printing exactly `300\n` and exiting 0. This is the entire behavior affected by the reorganization; no input validation, security, storage, or external communication changes are involved. The owner confirmed the observed legacy output as the compatibility contract before implementation. The records below are independent observations.

## Target

- Existing test ID: `tests/amount.test.js` / `three units retain the legacy total`. It starts `./bin/amount` with arguments `3`, `100` and asserts exactly `300\n` and exit code 0.
- Registered command:test: Executes that existing test with `node --test tests/amount.test.js`, timeout 30 seconds. It passes on the original and reorganized trees.
- Expectation source: The confirmed legacy behavior recorded in C1. Apply principle 3 of `principles/testing.md` only to this characterization expectation; do not infer new requirements from arbitrary implementation output.
- Detection: On the reorganized tree, changing only the returned total to 103 made the same test ID fail with expected `300\n`, actual `103\n`. The small change and run result were recorded together. Immediately restore the source; comparison confirmed exact restoration and the registered command passed again.
- Test selection: The existing test already detects the affected failure through the public entry point. No duplicate internal-helper test is added for the same guarantee under the same premises. No unrelated contract or failure condition is omitted.

