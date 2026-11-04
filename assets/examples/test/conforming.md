## Request

Review tests for a local amount-calculation CLI. Approved C1: `amount 3 100` prints exactly `300\n` and exits 0. Approved C2: `amount invalid 100` prints no standard output, prints exactly `Invalid quantity\n` to standard error, and exits 2. The CLI has no storage or external effects. The following artifacts and observations are fictional inputs independently captured from the target tree.

## Target

# Registered command

```toml
[commands.test]
executable = "node"
args = ["--test", "tests/amount.test.js"]
timeout_seconds = 30
```

# tests/amount.test.js

```javascript
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';

test('three units at 100 print the agreed total', () => {
  const result = spawnSync('./bin/amount', ['3', '100'], { encoding: 'utf8' });
  assert.equal(result.status, 0);
  assert.equal(result.stdout, '300\n');
});

test('invalid quantity returns the input error contract', () => {
  const result = spawnSync('./bin/amount', ['invalid', '100'], { encoding: 'utf8' });
  assert.equal(result.status, 2);
  assert.equal(result.stdout, '');
  assert.equal(result.stderr, 'Invalid quantity\n');
});
```

# Verification records

- Environment: Node.js with ES modules enabled, local CLI, no network or persistent data. Registered command:test executed both tests on the target tree and exited 0.
- Expected values: Taken from C1 and C2, not copied from implementation output. Tests observe the CLI process and its streams without calling internal helpers.
- Detection: Changing only the calculation result from multiplication to addition made `tests/amount.test.js` / `three units at 100 print the agreed total` fail with expected `300\n`, actual `103\n`. After immediate restoration, the source matched the original tree. Changing only the invalid-input exit code from 2 to 0 made `tests/amount.test.js` / `invalid quantity returns the input error contract` fail on the exit status. Restore immediately; source comparison confirmed restoration and both tests then passed.
- No internal reorganization is planned solely to demonstrate that tests survive one. These tests cover the changed contract and all behavior retained by this change.

