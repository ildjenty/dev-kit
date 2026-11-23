# Contract

What callers may supply, observe and rely on through failure, reruns and concurrency. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

Check public input and response contracts, failure classification, reruns and concurrency, and required external state and execution order under the owned principles. Coordinate with data rows on accepted valid states without taking ownership of that choice. Apply `every_quality` to your own rows: identify omitted hard-to-change choices made by the skeleton and check the timing and evidence of finalization under `designing.md#1`. Check decision assignments and approval against `authority.md`.

## Code

Check conformance to each contract row. Inspect failures for concealment, duplicate side effects and concurrent execution for invariant loss, and external prerequisites for missing state or ordering. Verify sent and received content and communication failure handling against the specification. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

## Blocking

A blocking condition of an owned principle applies, or sent or received content or communication failure handling differs from the specification. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Reconsidering accepted state choices already judged by data.
