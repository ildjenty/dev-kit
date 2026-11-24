# Readability

Which names, comments, conventions and shared rule definitions make the changed code understandable and keep rules that must change together consistent. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

No plan judge: this quality assesses the changed code.

## Code

Check conformance to each readability row. Apply `engineering.md#3` to names understandable to first-time readers and `designing.md#7` to existing conventions, error forms and log formats. List every added or changed comment and check which permitted kind or exemption in `engineering.md#4` applies; report comments fitting none. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

For each added function, conversion, check or rule, search the whole judged tree at the `tree` path in `judge/target.json` for existing code that already does the same. Apply `engineering.md#8`: report re-implemented rules or behaviour that must change together with an existing definition. Similar text alone is not a violation; keep processing independent when its reasons for change or contracts differ.

## Blocking

A blocking condition of an owned principle applies. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Placement or dependency choices justified only by existing layout, or stylistic preferences outside existing conventions.
