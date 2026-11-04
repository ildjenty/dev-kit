# State at termination

Compare the termination block of the round with the completion conditions and the handoff requirements for interrupted work.

## Criteria

Principle 5 of `principles/context.md` ("Finish by meeting the completion conditions; when interrupting, leave a state that can be resumed safely") is part of the criteria. The completion finalization rules in `stages/README.md` (section "Completion finalization") define what devkit handles instead of this review.

## What to check

Check the termination block (`schemas/progress.json`, the record that returns work results in a fixed format) of the round included in the judgment request. A round runs from the work instruction to the decision on its result. Compare the block with the completion conditions in the plan (`schemas/plan.json`) and with the handoff requirements for interrupted work in principle 5 of `principles/context.md`.

Aggregating required checks and confirming that the commit succeeded are left to devkit's completion finalization under the completion conditions; they are not part of this review.

## Blocking

A blocking condition of principle 5 of `principles/context.md` applies. Apply the same principle when required verification remains outstanding.

## Exemptions

The exemption defined in principle 5 of `principles/context.md` applies.
