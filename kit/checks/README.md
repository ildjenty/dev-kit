Stores aspect files supplied by the kit.
Parsing and judgment semantics belong in internal/check, detection test examples in kit/examples/, and repository-owned aspect files in .devkit/checks/.
For placement, see `docs/architecture/modules.md#kit`; for aspect handling, see `docs/architecture/modules.md#check`.

An aspect file is named `<aspect_id>.md`. After the first line, `# <aspect_id>`, it contains one line each for `- Checks: `, `- blocking: `, `- Do not report: `, and `- Output format: `, in that order. Values are Markdown through the end of the line; the field names and first line are machine-readable. The corresponding aspect in `docs/checks.md` is the source of truth. When changing an aspect, update the corresponding definition and examples in `kit/examples/<aspect_id>/` together. Code-formatted references starting with `docs/` or `principles/` are relative to the repository root during development and to the kit copy root after installation. Follow [Detection test examples](../examples/README.md) for the test format.

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
