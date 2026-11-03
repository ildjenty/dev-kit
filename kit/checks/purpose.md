# purpose

- Checks: Check whether the relationship between the purpose and completion conditions can be explained briefly and whether completion can be judged.
- blocking: Completion cannot be judged.
- Do not report: Items outside this aspect's checks. Findings without a failure scenario.
- Output format: Follow the Judgment JSON Schema supplied in the request and the Judgment and Finding definitions in `docs/architecture/data.md`. Return `NOT ASSESSED` when necessary criteria or required verification are unavailable, and record the verified and unverified scope, method, and environment. Do not return findings without a failure scenario, and do not assign severity. Retain all findings; treat nonblocking findings as reference information without requiring fixes. Compare exemption claims with their grounds, and waive only the requirements covered by the exemption.
