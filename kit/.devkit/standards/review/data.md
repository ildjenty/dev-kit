# Data

How facts, valid states, ownership and lifetime preserve meaning and invariants. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

Check the source of truth, state transitions and recovery for failure, duplication and interruption; valid types and boundary validation; domain rules; ownership and lifetime; concurrent updates and integration; and representation or conversion meaning, under the owned principles. Apply `every_quality` to your own rows: identify omitted hard-to-change choices made by the skeleton and check the timing and evidence of finalization under `designing.md#1`. Check decision assignments and approval against `authority.md`.

## Code

Check conformance to each data row. Inspect implementation of state transitions, recovery, validation, invariants, concurrency, conversions and preservation of historical facts under the owned principles. Compare saved and retrieved content with the specification. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

## Blocking

A blocking condition of an owned principle applies, or saved content or retrieved results differ from the specification. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Contract failure responses or security permissions owned by other qualities.
