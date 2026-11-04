## Request

Review tests for a local amount-calculation CLI. Approved C1: `amount 3 100` prints exactly `300\n` and exits 0. The CLI has no storage or external effects. Treat the artifacts and observation below as independently captured.

## Target

- Registered command:test: Executes `node --test tests/amount.test.js` with a 30-second timeout.
- Only test: Calls the internal multiplication helper with 3 and 100 and asserts 300, as required by C1. It never starts the CLI.
- Observation: The test passes even though the CLI argument parser swaps quantity for a constant, prints a debug prefix, and returns exit code 1. No test observes the CLI's arguments, standard output, or exit code.

