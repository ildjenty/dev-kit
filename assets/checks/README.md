Aspect files define checks, blocking conditions, exclusions, and judgment output requirements. Repository-owned aspects are in `.devkit/checks/`; kit-provided aspects are in this directory.

An aspect file is named `<aspect_id>.md`. After the first line, `# <aspect_id>`, it contains one line each for `- Checks: `, `- blocking: `, `- Do not report: `, and `- Output format: `, in that order. Values are Markdown through the end of the line; the field names and first line are machine-readable. Keep an aspect and its detection examples in `examples/<aspect_id>/` consistent. Code-formatted references starting with `principles/` or a kit directory such as `schemas/`, `checks/`, or `rules/` are relative to the installed kit copy root. In the source repository, kit directories are under `kit/`, while `principles/` is at the repository root. Follow [Detection test examples](../examples/README.md) for the test format.

- `purpose.md`: Purpose.
- `irreversible.md`: Irreversible items.
- `structure.md`: Structure.
- `consistency.md`: Consistency.
- `wording.md`: Wording.
- `contract.md`: Contracts.
- `steps.md`: Steps.
- `diff.md`: Diff.
- `behavior.md`: Behavior verification.
- `completion.md`: State at termination.
- `plan.md`: Plan.
- `premise.md`: Premise critique.
- `harness.md`: Harness.
- `skill-test.md`: Skill tests.

## Judgment and findings

Return judgments using `schemas/judgment.json` and findings using `schemas/finding.json`, or the output schema supplied in the request. Use the requested instruction ID, aspect, target version, frozen norms, and completion condition IDs. A judgment is valid only for its matching target version and norms. The target version identifies a tree snapshot, plan version, or structure content hash; use the supplied value rather than substituting the current working tree.

Return `NOT ASSESSED` when necessary criteria or required verification are unavailable. Record checked and unchecked scope, method, and environment. Findings need a failure scenario and carry no severity. Use `auto-fixable` for findings that need no human decision and `needs-decision` for those that do. Retain all findings; nonblocking findings are reference information and do not require fixes. Compare exemption claims with their grounds and waive only the requirements covered by the exemption. Aggregate results in the order `FAIL`, `NOT ASSESSED`, then `PASS`. The author's report does not replace independent judgment. For diff checks, compare the supplied valid approvals and delegations with the target and scope.

## Detection tests

Only aspects handled by judge subagents or judgment commands require detection tests; `command:` checks do not. A required aspect needs a pass for the runner assigned by the plan. Test violation, conforming, and allowed examples using `examples/README.md`; return observations using `schemas/detection-result.json`. Expected results alone are not passes.

Pass identity consists of the content hashes of the aspect, specification including example bodies and expected judgments, and assigned runner definition. Store the pass index in `.devkit/detect.json`; execution records retain the execution facts. Any change to those hashes requires another test, including wording changes. Model or effort observations are provenance, not pass identity. Devkit requests missing tests before opening a round; failed tests stop for a decision to revise the aspect or remove its required status. The manual-test exemption in principle 2 of `principles/harness.md` applies only to human-run tests.
