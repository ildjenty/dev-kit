# Behavior verification

Compare the observed behavior of the change with its specification.

## Criteria

The screen rules in `rules.md` (section "Screens") and principle 9 of `principles/ui.md` ("Check rendering results and interaction") are part of the criteria.

## What to check

- **General**: Startup, interaction, console errors, evidence capture, and cleanup.
- **Data changes**: Whether saved content and retrieved results match the specification.
- **External communication changes**: Whether sent and received content and failure handling match the specification.
- **UI changes**: Follow the screen rules in `rules.md`. Check rendering and interaction under principle 9 of `principles/ui.md`, including data prone to problems, such as long names, no results, large datasets, and multiple languages.

## Blocking

Saved content, retrieved results, sent or received content, or communication failure handling differs from the specification. For UI changes, a blocking condition of principle 9 of `principles/ui.md` or a violation of the screen rules also blocks. Handle unverified items under the same principle.

## Exemptions

The exemptions defined in principle 9 of `principles/ui.md` and in the screen rules apply.
