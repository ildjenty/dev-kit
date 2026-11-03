# premise

- Checks: Check whether simpler alternatives exist for the number and format of components, principles, records, and aspects. Follow principle 5 of `principles/harness.md` (`principles/harness.md#5-%E9%98%B2%E3%81%90%E5%A4%B1%E6%95%97%E3%81%A8%E9%81%8B%E7%94%A8%E8%B2%A0%E6%8B%85%E3%81%8B%E3%82%89%E4%BB%95%E7%B5%84%E3%81%BF%E3%82%92%E9%81%B8%E3%81%B6`) for comparisons.
- blocking: Only a blocking condition in principle 5 makes this aspect blocking.
- Do not report: Items outside this aspect's checks. Findings without a failure scenario.
- Output format: Follow the Judgment JSON Schema supplied in the request and the Judgment and Finding definitions in `docs/architecture/data.md`. Return `NOT ASSESSED` when necessary criteria or required verification are unavailable, and record the verified and unverified scope, method, and environment. Do not return findings without a failure scenario, and do not assign severity. Retain all findings; treat nonblocking findings as reference information without requiring fixes. Compare exemption claims with their grounds, and waive only the requirements covered by the exemption.
