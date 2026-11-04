# irreversible

- Checks: Check the list of irreversible items (decisions that are difficult to change) for omissions. Follow principle 1 of `principles/designing.md` (`principles/designing.md#1-identify-decisions-that-are-hard-to-change-early-and-finalize-them-after-obtaining-the-evidence-needed`) for when to decide and finalize them.
- blocking: The list has an omission, or a blocking condition in principle 1 applies.
- Do not report: Items outside this aspect's checks. Findings without a failure scenario.
- Output format: Follow the Judgment JSON Schema supplied in the request (`schemas/judgment.json`) and `schemas/finding.json`, with the judgment rules in `checks/README.md`. Return `NOT ASSESSED` when necessary criteria or required verification are unavailable, and record the verified and unverified scope, method, and environment. Do not return findings without a failure scenario, and do not assign severity. Retain all findings; treat nonblocking findings as reference information without requiring fixes. Compare exemption claims with their grounds, and waive only the requirements covered by the exemption.
