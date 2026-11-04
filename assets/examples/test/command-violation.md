## Request

Review tests for a local amount-calculation CLI. Approved C1: `amount 3 100` prints exactly `300\n` and exits 0. The CLI has no storage or external effects. Treat the artifacts and observation below as independently captured.

## Target

- tests/amount.test.js: Spawns the actual CLI with arguments `3`, `100` and asserts standard output `300\n` and exit code 0. Expectations come from C1.
- devkit.toml: Contains no registered commands.
- Observation: A developer manually ran `node --test tests/amount.test.js` successfully. No registered command executes the test.
- Report: The manual invocation is treated as sufficient to make the test part of required command:test.

