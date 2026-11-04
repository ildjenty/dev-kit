# Detection examples

Detection examples verify whether a check's criteria produce the expected judgments with its assigned runner. A judge-run check selects its examples in the plan template from the `examples` source, for example `"examples": {"source": "examples", "select": "test/spec.json"}`. The selection names a specification; the specification names its example files. `command:` checks have no examples.

## Requirement

With `[detection] required = true` in `devkit.toml` (the default), each required check handled by a judge subagent or judgment command needs a detection pass for the runner assigned by the plan; `command:` checks do not. A required judge-run check without examples cannot satisfy this requirement. Manual evaluation does not exempt a required judge-run check. With `required = false`, devkit requests no detection tests and accepts judgments without a pass record. This is the detection requirement referenced by stages and principle 2 of `principles/harness.md`.

When the detection requirement applies, devkit requests missing tests before opening a round. A failed test stops for a decision to revise the check's criteria or examples, or to remove its required status.

## Specification and examples

`<check>/spec.json` contains `aspect` and `cases`. Each case contains `kind` (`violation`, `conforming`, or `allowed`), `file` relative to the specification, and `expected` (`FAIL`, `PASS`, or `PASS`, respectively). Each example is Markdown with `## Request` and `## Target`, using fictional inputs, artifacts, and observation records.

## Running a test

The judge receives only the criteria documents selected by the check, opaque example IDs with the example bodies, selected input contract schemas, and the output contract, and returns observations using `schemas/detection-result.json`; each observation contains a judgment following `schemas/judgment.json` and returns the opaque ID in its `file` field. The specification, case kinds, file names, expected judgments, the ID-to-specification mapping, and any work history or reference that contains them are withheld from the judge; they stay in the regular work history. Run tests with the runner assigned by the plan.

Devkit keeps the ID-to-specification mapping and compares the returned verdicts with the expectations. A matching verdict does not establish that a violation failed for the intended reason; findings and grounds remain available for review. Conforming and allowed examples must not be blocked by ignoring relevant differences or supported exemptions. Listing an expected judgment is not a pass record; a pass is recorded only after the actual judgment and its grounds are obtained.

## Pass identity

Pass identity consists of the assigned runner definition, the judgment request template including its judgment rules, the content hashes of the documents actually supplied as criteria, and the content hashes of the specification and its example files. Any change to them requires another test, including wording changes. Model or effort observations are provenance, not pass identity. The pass index is stored at `[state] detect` (default `.devkit/detect.json`); execution records keep the execution facts.
