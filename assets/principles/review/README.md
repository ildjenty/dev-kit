# Review documents

A judge-run check names its review criteria in the plan template as selections from knowledge sources, for example `"criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}]`. The documents in `principles/review/` state, for one kind of review, what to check, which other documents are part of the criteria, what blocks, what not to report, and which exemptions apply. The check name (`aspect`) only names the judgment; devkit does not read a review document's structure.

Devkit does not follow references inside a document. A check that selects a review document also selects every document that the review names as part of its criteria, from the source that holds it. Only norm sources can supply criteria. Schemas named as `schemas/…` are owned by the binary and supplied in the request; they are not selected.

The output format and the rules common to every judgment (`NOT ASSESSED`, failure scenarios, no severity, nonblocking findings, exemption handling, aggregation) are in the judgment request template `templates/brief/judgment.md.tmpl`. Review documents do not repeat them.

The repository owns and maintains these documents. A change to a review document is a loop edit.

- `principles/review/purpose.md`: Purpose.
- `principles/review/irreversible.md`: Irreversible items.
- `principles/review/structure.md`: Structure.
- `principles/review/consistency.md`: Consistency.
- `principles/review/wording.md`: Wording.
- `principles/review/contract.md`: Contracts.
- `principles/review/steps.md`: Steps.
- `principles/review/diff.md`: Diff.
- `principles/review/test.md`: Tests of completion conditions and public contracts.
- `principles/review/behavior.md`: Behavior verification.
- `principles/review/completion.md`: State at termination.
- `principles/review/plan.md`: Plan.
- `principles/review/harness.md`: Harness, retrospective, and mechanism selection.
- `principles/review/skill-test.md`: Skill tests.
