## Request

Check the proposal to add a mechanism preventing broken document links. Treat the guarantees and burdens of each compared proposal as verified in the records below.

## Target

# Selecting required verification

- Failure to prevent: Broken links on document changes recur about once a month, making references inaccessible.
- Proposal A: Use an existing command on document changes to verify all links after the change. It can always run before commit and takes about one second. No additional maintenance or false positives.
- Proposal B: Launch a judge on every change to check the same links. About 30 seconds, with role definition maintenance and false-positive handling. Detection coverage and stopping point are the same as A.
- Decision: Require B on every change. There is no reason to choose the proposal with greater burden than A.
