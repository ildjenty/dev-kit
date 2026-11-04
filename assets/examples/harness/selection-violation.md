## Request

Check the proposal to prevent broken document links. Treat the guarantees and burdens below as independently verified. Existing judgment processing, skill, contracts, and fresh-session instructions are unchanged. This request concerns mechanism selection, not completion of a retrospective.

## Target

# Selecting required verification

- Failure to prevent: Broken links on document changes recur about once a month, making references inaccessible.
- Proposal A: Use an existing command on document changes to verify all links before commit. About one second, no additional maintenance, and no false positives.
- Proposal B: Launch a judge on every change to check the same links. About 30 seconds, with role definition maintenance and false-positive handling. Detection coverage and stopping point are the same as A.
- Decision: Require B on every change. There is no reason to choose the proposal with greater burden than A.

