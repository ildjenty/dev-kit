# consistency

- Checks: Check documents and design proposals for contradictions, undefined matters, and terminology differences that lead to different meanings, implementations, or decisions.
- blocking: There is a contradiction. Record undefined matters and terminology differences that lead to different meanings, implementations, or decisions without making them blocking.
- Do not report: Items outside this aspect's checks. Findings without a failure scenario.
- Output format: Follow the Judgment JSON Schema supplied in the request (`schemas/judgment.json`) and `schemas/finding.json`, with the judgment rules in `checks/README.md`. Return `NOT ASSESSED` when necessary criteria or required verification are unavailable, and record the verified and unverified scope, method, and environment. Do not return findings without a failure scenario, and do not assign severity. Retain all findings; treat nonblocking findings as reference information without requiring fixes. Compare exemption claims with their grounds, and waive only the requirements covered by the exemption.
