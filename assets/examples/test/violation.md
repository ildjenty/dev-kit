## Request

Review tests for a local amount-calculation CLI. Approved C1: `amount 3 100` prints exactly `300\n` and exits 0. Approved C2: `amount invalid 100` prints no standard output, prints exactly `Invalid quantity\n` to standard error, and exits 2. The CLI has no storage or external effects. Treat the artifacts and observation below as independently captured.

## Target

- Registered command:test: Executes `node --test tests/amount.test.js` with a 30-second timeout.
- Only test in tests/amount.test.js: Spawn `./bin/amount` with arguments `3`, `100`; assert exit code 0 and standard output `300\n`, using C1 as the expectation.
- Observation: That test passed. There are no tests or alternative verification of invalid input, its output streams, or its exit code.
- Report: The passing command is treated as evidence that C1 and C2 are both covered.

