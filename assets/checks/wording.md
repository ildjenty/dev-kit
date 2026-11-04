# wording

- Checks: Check assumptions about the intended reader and the specificity of wording against the violations and warning signs in principles 2 and 3 of `principles/writing.md`.
- blocking: A passage causes the reader to misunderstand its meaning.
- Do not report: Items outside this aspect's checks. Findings without a failure scenario.
- Output format: Follow the Judgment JSON Schema supplied in the request (`schemas/judgment.json`) and `schemas/finding.json`, with the judgment rules in `checks/README.md`. Return `NOT ASSESSED` when necessary criteria or required verification are unavailable, and record the verified and unverified scope, method, and environment. Do not return findings without a failure scenario, and do not assign severity. Retain all findings; treat nonblocking findings as reference information without requiring fixes. Compare exemption claims with their grounds, and waive only the requirements covered by the exemption.
