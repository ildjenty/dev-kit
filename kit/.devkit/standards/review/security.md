# Security

Which trust boundaries, permissions, dependencies and records protect operations and information. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

Check trust boundaries, secret handling, authentication and authorization of each operation, least privilege and safe defaults, deliberately selected and pinned dependencies, security event records, and the authority and information passed to services or processes, under the owned principles. Apply `every_quality` to your own rows: identify omitted hard-to-change choices made by the skeleton and check the timing and evidence of finalization under `designing.md#1`. Check decision assignments and approval against `authority.md`.

## Code

Check conformance to each security row. Inspect actual input handling, secrets in code/test data/logs/reports, server-side authentication and authorization on every changed operation, permissions and defaults, executed dependencies, event records and information passed externally under the owned principles. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

## Blocking

A blocking condition of an owned principle applies. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Additional security mechanisms without a failure or threat in the changed path.
