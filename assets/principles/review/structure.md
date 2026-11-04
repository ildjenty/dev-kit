# Structure

Review module placement, dependencies, boundaries, and style against the design, structure, and data principles.

## Criteria

All principles in `principles/designing.md`, `principles/structure.md`, and `principles/data.md` are part of the criteria.

## What to check

- **Placement**: Whether each placement can be justified by its responsibility.
- **Principles**: Violations or warning signs under the principles in `principles/designing.md`, `principles/structure.md`, and `principles/data.md`.
- **Simplicity**: Follow principle 7 of `principles/structure.md` ("Reduce the layers to follow and the state to remember") for comparing alternatives.
- **Boundaries**: Follow principle 2 of `principles/structure.md` ("Do not repeat validation whose preconditions are maintained") for validating external inputs, and principle 5 of `principles/data.md` ("Define data ownership and lifetime, and the boundaries that keep invariants") for consistency during updates.
- **Style**: Follow principle 7 of `principles/designing.md` ("Align with existing conventions") for aligning structure.

## Blocking

A blocking condition of a principle named above applies.

## Exemptions

The exemptions defined in the principles named above apply.
