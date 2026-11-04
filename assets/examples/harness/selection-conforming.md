## Request

Check the proposal to prevent broken document links. Treat the guarantees and burdens below as independently verified. Existing judgment processing, skill, contracts, and fresh-session instructions are unchanged. This request concerns mechanism selection, not completion of a retrospective.

## Target

# Selecting required verification

- Failure to prevent: Broken links on document changes recur about once a month, making references inaccessible.
- Proposal A: Use an existing command on document changes to verify all links before commit. About one second, no additional maintenance, and no false positives.
- Proposal B: Have a judge check the same links on every change. About 30 seconds, with role definition maintenance and false-positive handling. The guarantee is the same as A.
- Decision: Require A only on document changes, retaining necessary detection coverage and stopping point while reducing burden. Remove the judge's duplicate check.

