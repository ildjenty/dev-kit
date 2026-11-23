# Delivery

How ordered release units and verification cover the request end to end. Read the principle references in the supplied materials; their applicability and exemptions remain in force.

## Decisions

Check ordered release units and the test plan under `designing.md#3` and `working.md#2`. The skeleton precedes implementation and has verifiable conditions. The first unit connects the user entry point through to data, including startup and tests; one unit suffices for a small task. Each unit has completion condition IDs, scope, verification method and size grounds showing it fits one implement instance. Implementation is followed by verification before the next unit. Compare the assigned IDs with each unit and ensure final acceptance covers every condition; the final demo can serve as acceptance. Required investigation for UI, data, external communication or structure is included. Each completion condition has a verification method (command, judge, judgment-command check or owner approval point). Check overlapping scopes for concrete collisions; exclude completed units when comparing remaining work. Apply `every_quality` to your own rows: identify omitted hard-to-change choices made by the skeleton and check the timing and evidence of finalization under `designing.md#1`. Check decision assignments and approval against `authority.md`.

## Code

No code judge: implementation evidence is assessed by the code qualities.

## Blocking

A completion condition is uncovered or lacks a verification method; the skeleton has no verifiable condition or implementation precedes it; implementation and verification are out of order; a demo lacks condition IDs or acceptance fails to cover all conditions; an increment cannot be verified or fit within one implement instance; overlapping unit scopes can cause conflicting changes; or a blocking condition of an owned principle applies.

## Not reported

Missing quality rows and hard-to-change decision timing belong to each quality under `every_quality`. Structural placement, authority assignment and scope conformance belong to their respective qualities. Differences from a reference pattern alone do not block.
