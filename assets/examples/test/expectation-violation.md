## Request

Review tests for a local amount-calculation CLI. Approved C1: `amount 3 100` prints exactly `300\n` and exits 0. The CLI has no storage or external effects. Treat the artifacts and observation below as independently captured.

## Target

- Registered command:test: Executes `node --test tests/amount.test.js` with a 30-second timeout.
- Only test: Starts the CLI twice with arguments `3`, `100`; uses the first run's standard output and exit code as the expected values for the second run. It has no expectation derived from C1.
- Observation: Both runs print `103\n` and exit 0. The equality assertions pass, and changing the calculation to another deterministic wrong result still passes.
- Coverage: No other test checks the required total or the specified exit code.

