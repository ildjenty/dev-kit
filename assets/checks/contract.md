# contract

- Checks: Follow principle 3 of `principles/data.md` (`principles/data.md#3-prevent-invalid-states-and-values-with-types-and-validation`) for accepting states and values, and principle 1 of `principles/engineering.md` (`principles/engineering.md#1-do-not-hide-failures`) for failure classification and public responses.
- blocking: A blocking condition in a referenced principle applies.
- Do not report: Items outside this aspect's checks. Findings without a failure scenario.
- Output format: Follow the Judgment JSON Schema supplied in the request (`schemas/judgment.json`) and `schemas/finding.json`, with the judgment rules in `checks/README.md`. Return `NOT ASSESSED` when necessary criteria or required verification are unavailable, and record the verified and unverified scope, method, and environment. Do not return findings without a failure scenario, and do not assign severity. Retain all findings; treat nonblocking findings as reference information without requiring fixes. Compare exemption claims with their grounds, and waive only the requirements covered by the exemption.
