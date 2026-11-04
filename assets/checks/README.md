Aspect files define checks, blocking conditions, exclusions, and judgment output requirements. The repository owns and maintains its aspects and detection examples.

An aspect file is named `<aspect_id>.md`. After the first line, `# <aspect_id>`, it contains one line each for `- Checks: `, `- blocking: `, `- Do not report: `, and `- Output format: `, in that order. Values are Markdown through the end of the line; the field names and first line are machine-readable. Keep an aspect and its detection examples in `checks/<aspect_id>/` consistent. References such as `principles/…`, `stages/…`, `checks/…`, `plans/…`, `templates/…`, `rules.md`, `requirements.md`, and `structure.json` are logical names resolved through `[paths]` in `devkit.toml`; briefs supply the mapping to snapshot paths. For `schemas/…`, devkit supplies the output schema body in the brief.

- `checks/purpose.md`: Purpose.
- `checks/irreversible.md`: Irreversible items.
- `checks/structure.md`: Structure.
- `checks/consistency.md`: Consistency.
- `checks/wording.md`: Wording.
- `checks/contract.md`: Contracts.
- `checks/steps.md`: Steps.
- `checks/diff.md`: Diff.
- `checks/test.md`: Tests of completion conditions and public contracts.
- `checks/behavior.md`: Behavior verification.
- `checks/completion.md`: State at termination.
- `checks/plan.md`: Plan.
- `checks/harness.md`: Harness, retrospective, and mechanism selection.
- `checks/skill-test.md`: Skill tests.

## Judgment and findings

Return judgments using `schemas/judgment.json` and findings using `schemas/finding.json`, or the output schema supplied in the request. Use the requested instruction ID, aspect, target version, frozen norms, and completion condition IDs. A judgment is valid only for its matching target version and norms. The target version identifies a tree snapshot, plan version, or structure content hash; use the supplied value rather than substituting the current working tree. Read norms from the supplied snapshot paths, not from norms being edited in the working tree. Round judgments use round-start norms; plan checks and detection tests use committed norms from the work branch HEAD fixed at instruction issue time. For a round judgment, inspect the expanded round-start comparison base, changed-file list, and patch to the returned target referenced by the request. Every judgment request also references the work-history file fixed at instruction issue time; use it as required by the aspect.

Return `NOT ASSESSED` when necessary criteria or required verification are unavailable. Record checked and unchecked scope, method, and environment. Findings need a failure scenario and carry no severity. Use `auto-fixable` for findings that need no human decision and `needs-decision` for those that do. Retain all findings; nonblocking findings are reference information and do not require fixes. Compare exemption claims with their grounds and waive only the requirements covered by the exemption. Aggregate results in the order `FAIL`, `NOT ASSESSED`, then `PASS`. The author's report does not replace independent judgment. For diff checks, compare the supplied valid approvals and delegations with the target and scope.

## Command checks

If `command:<name>` is absent from the round-start `devkit.toml`, return `FAIL` for that check with an `auto-fixable` finding: "Verification command `<name>` is not registered. Create a verification method and register it in `[commands.<name>]` in `devkit.toml`." Registration is a loop edit, follows the human confirmation policy, and takes effect from the next round's snapshot. If a command changes the tree, its finding must also instruct the worker to add generated output paths to the repository's Git ignore rules.

## Detection tests

With `[detection] required = true` in `devkit.toml` (the default), each required aspect handled by a judge subagent or judgment command needs a detection pass for the runner assigned by the plan; `command:` checks do not. Manual evaluation does not exempt a required judge-run aspect. With `required = false`, devkit requests no detection tests and accepts judgments without a pass record. This is the detection requirement referenced by stages and principle 2 of `principles/harness.md`. When testing, use violation, conforming, and allowed examples described below and return observations using `schemas/detection-result.json`. Expected results alone are not passes.

`checks/<aspect_id>/spec.json` contains `aspect` and `cases`. Each case contains `kind` (`violation`, `conforming`, or `allowed`), `file` relative to the specification, and `expected` (`FAIL`, `PASS`, or `PASS`, respectively). Each example body contains `## Request` and `## Target`. Give the judge only opaque example IDs and bodies as test input, together with frozen norms and the supplied output schema; withhold the specification, case kind, file name, and expected judgment. Return each opaque ID in the observation's `file` field. Devkit keeps the ID-to-specification mapping and compares the returned judgment with the expectation. Devkit checks whether returned verdicts match expectations; it does not verify that a violation failed for the intended reason. Findings and grounds remain available for review. Conforming and allowed examples must not be blocked by ignoring relevant differences or supported exemptions.

Pass identity consists of the content hashes of the aspect, the `principles/<name>.md` files and `rules.md` it references, the specification including example bodies and expected judgments, and the assigned runner definition. Store the pass index at the configured detection index path; execution records retain the execution facts. Any change to those hashes requires another test, including wording changes. Model or effort observations are provenance, not pass identity. When the detection requirement applies, devkit requests missing tests before opening a round; failed tests stop for a decision to revise the aspect or remove its required status.
