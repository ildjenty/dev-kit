# Cost

Which performance, observability, rollout and operational measures justify their cost. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

Apply `designing.md#6` to the performance, observability, rollout and threat-model measures needed by this change. Check the evidence, expected load, recurring usage, resource needs and operational burden that justify the chosen measures. Apply `authority.md` for unresolved cost and operational decisions. Apply `every_quality` to your own rows: identify omitted hard-to-change choices made by the skeleton and check the timing and evidence of finalization under `designing.md#1`. Check decision assignments and approval against `authority.md`.

## Code

Check conformance to each cost row and whether the actual calls, resources, controls, observability and rollout implement the chosen measures under `designing.md#6`. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

## Blocking

A blocking condition of an owned principle applies, or the implementation exceeds the agreed cost or operational scope. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Speculative optimization without evidence of a relevant failure or load.
