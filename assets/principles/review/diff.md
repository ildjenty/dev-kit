# Diff

Review the code, the scope of decisions and operations, the style, the PR body, and the report of a diff.

## Criteria

The following are part of the criteria:

- Principles 1 ("Do not hide failures") and 5 ("Investigate where a change has impact and errors with the same cause") of `principles/engineering.md`.
- Principles 6 ("Make execution decisions within agreed and delegated scope, and request required approval") and 7 ("Do not make unrequested changes") of `principles/working.md`.
- Principle 7 of `principles/designing.md` ("Align with existing conventions").
- Principle 1 of `principles/writing.md` ("Write the conclusion and what people need for decisions first").
- Principle 1 of `principles/verification.md` ("Do not write unverified things as if verified").
- The PR body template `templates/pr.md.tmpl`.
- The writing and records rules in `rules.md` (section "Writing and records").

## What to check

- **Code**: Follow principles 1 and 5 of `principles/engineering.md` for failures and change impact.
- **Approval and delegation**: Follow principles 6 and 7 of `principles/working.md` for the scope of decisions and operations. Compare the target and its scope with the valid approvals and delegations included in the judgment request.
- **Style**: Follow principle 7 of `principles/designing.md` for aligning the diff.
- **PR body**: Follow the PR body template `templates/pr.md.tmpl`, preserving existing repository headings, and the writing and records rules in `rules.md`.
- **Reporting**: Follow principle 1 of `principles/writing.md` for the information needed for decisions, and principle 1 of `principles/verification.md` for verification claims. Apply both principles to PR bodies as well.

## Blocking

A blocking condition of a principle named above applies.

## Exemptions

The exemptions defined in the principles named above apply.
