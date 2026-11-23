# Structure

Where responsibilities live and how boundaries and dependencies contain change. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

Check placement, responsibilities, dependency direction and boundaries using `structure/purpose.md`, `structure/deciding.md`, `structure/smells.md` and the owned principles. Compare alternatives by the knowledge hidden, change reach, layers traversed and state remembered. The skeleton and decision evidence must justify each placement. Each new structure row supplies `nature`: kind, reasons for change (who changes it and why), volatility and callers, each supported by a path, request words or a call relation, or explicitly unknown. Check that placement follows from this nature; an unknown must not be treated as established evidence. Existing layout alone does not justify placement. Apply `every_quality` to your own rows: identify omitted hard-to-change choices made by the skeleton and check the timing and evidence of finalization under `designing.md#1`. Check decision assignments and approval against `authority.md`.

At both plan and code, inspect the level receiving the new items and its full ancestor path for smells, including pre-existing ones, as `structure/deciding.md` requires. Report smells elsewhere with `deferred: true`, `blocking: false` and `needs_decision: false`. They go to the backlog. Use `outside_plan` and top-level and second-level folder counts to check locality; spread the plan did not declare needs a reason in the code or a finding.

## Code

Apply the path-of-change smell and locality review in Decisions at code time as well. Check conformance to the structure rows and examine function-body dependencies and guarantees. Compare the fixed before-and-after facts and diff for architecture smells in `structure/smells.md`. Apply the owned principles to abstraction, migrations, removal, universal guarantees, external types and validation boundaries. A choice passed at plan time is checked for conformance, not judged again as a choice. Without a plan, assess the supplied `without_plan` choices using Decisions and `every_quality`.

## Blocking

A blocking condition of an owned principle applies, or an architecture smell in `structure/smells.md` lies on the path of the change, including a pre-existing smell. Deferred smells elsewhere never block. A blocking condition of the supplied `every_quality` principles also applies to this quality’s rows.

## Not reported

Data rules owned by data, style owned by readability, or a difference from a reference layout without a concrete structural failure.
