# Tests of completion conditions and public contracts

Review whether the completion conditions assigned to the check and the user-visible contracts are covered by concrete tests that run through a registered command.

## Criteria

All principles in `principles/testing.md` are part of the criteria.

## What to check

- **Coverage of completion conditions**: For each completion condition assigned to this check, identify concrete tests and check whether they run through a command registered in `devkit.toml`. Inspect the tests and the command's execution path, not only a passing command or the author's report.
- **Entry point**: Verify contracts through the user-visible entry point. For a CLI, check arguments, standard output, standard error where specified, and exit codes.
- **Expectations and independence from implementation**: Check that expected values come from the agreed contract, and that tests observe behavior rather than reproduce implementation logic or assert internal details outside the contract.
- **Test selection, failure detection, existing behavior, and changed contracts**: Apply the requirements and exemptions of `principles/testing.md`.

## Blocking

A completion condition assigned to this check has no concrete test executed through a registered command, the relevant user-visible contract is untested, or a blocking condition of `principles/testing.md` applies.

## Not reported

Requests for additional tests that would duplicate the same guarantee under the same premises without detecting a different failure.

## Exemptions

The exemptions defined in `principles/testing.md` apply.
