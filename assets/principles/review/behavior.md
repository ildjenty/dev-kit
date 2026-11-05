# Behavior verification

Compare the observed behavior of the change with its specification.

## Criteria

The screen rules in `rules.md` (section "Screens") and principle 9 of `principles/ui.md` ("Check rendering results and interaction") are part of the criteria.

## What to check

- **Demo scope**: Compare the report with the completion condition IDs assigned to this verification instance's checks. Check that correction rounds stay within the approved design and conditions rather than absorbing changes to either.
- **General**: Startup, interaction, console errors, evidence capture, and cleanup.
- **Data changes**: Whether saved content and retrieved results match the specification.
- **External communication changes**: Whether sent and received content and failure handling match the specification.
- **UI changes**: Follow the screen rules in `rules.md`. Check rendering and interaction under principle 9 of `principles/ui.md`, including data prone to problems, such as long names, no results, large datasets, and multiple languages.

## Blocking

An assigned condition is not demonstrated, a correction absorbs an unapproved requirement or design change, or saved content, retrieved results, sent or received content, or communication failure handling differs from the specification. For UI changes, a blocking condition of principle 9 of `principles/ui.md` or a violation of the screen rules also blocks. Handle unverified items under the same principle.

## Exemptions

The exemptions defined in principle 9 of `principles/ui.md` and in the screen rules apply.
