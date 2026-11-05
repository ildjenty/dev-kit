# Harness, retrospective, and mechanism selection

Review the loop's mechanisms: its principles and contracts, the mechanisms made required, the devkit skill, judgment handling, principle content, whether a fresh session can pick up the work, and retrospectives.

## Criteria

The following are part of the criteria:

- All principles in `principles/harness.md`.
- Principle 5 of `principles/working.md` ("Turn routine processing into commands and have procedures call them").
- The runtime rules in `stages/README.md`, `stages/retrospective.md`, `plans/README.md`, and `principles/review/README.md`.
- The judgment request template `templates/brief/judgment.md.tmpl`, which holds the common judgment rules.
- The applicable schemas in `schemas/`, which devkit supplies in the request.

## What to check

- **Principles and contracts**: Follow the principles in `principles/harness.md` and the applicable schemas and runtime rules named above.
- **Required checks**: Follow principle 4 of `principles/harness.md` for selecting mechanisms.
- **Skill**: Check for a `description` (`https://agentskills.io/specification#description-field`, the skill description field), placement (the canonical devkit skill is `.agents/skills/devkit/`, linked from `.claude/skills/devkit`), and references. Follow principle 5 of `principles/working.md` for dividing work between implementation and procedures.
- **Judgment**: Check whether `NOT ASSESSED` can be returned under the check results contract (`schemas/judgment.json` and the judgment rules in `templates/brief/judgment.md.tmpl`).
- **Principle content**: Check whether applicability, requirements, exceptions, and blocking conditions can be understood.
- **Fresh session**: Check whether reading the repository and execution records answers what the system is, how to run it, how to verify it, and where the work currently stands.
- **Retrospective**: When assessing a retrospective, read the work-history file referenced in the brief, fixed when the instruction was issued, and follow `stages/retrospective.md`. Compare the review with that history and check that it considers all judgments and findings, including nonblocking findings, `FAIL` and `NOT ASSESSED` outcomes, round counts, human rejections, corrections and replanning instructions, missed issues reported as lessons through note, the complete human-input list including chat outside question answers, and the norms inventory. For each problem, check where it should have been caught and the destination for preventing recurrence, following `[lessons] destinations` in `devkit.toml`. The default order is machine checks (hooks, lint, types, tests, and verification commands), plan templates, review documents in `principles/review/` (the `aspect` destination), principles, and the skill. Check that repository-owned norms are updated, or that a reason for no update is recorded. Consider unenforced items in `rules.md` and recurring violations as candidates for machine checks. Check that approved completion conditions that constitute repository requirements are reflected in `requirements.md` unless purpose already covers them. Check that decisions needed by later work are kept in `requirements.md`, `structure.json`, or `rules.md` as appropriate, not only in execution records.
- **Decision boundaries**: Compare the retrospective with every human input. Check that it accounts for all human stopping points, whether the human found them necessary, and decisions that should have been raised but were not. Follow `stages/retrospective.md` for changes or reasons for no change, including delegated decision kinds and their grounds. When removing a gate, check that both the pattern and loop rule change together.
- **Norms and burden**: Check how the relevant principles are used by review documents, check criteria, and plan templates, whether mechanisms serve their purpose, and whether any mechanism is unused or costs more than it warrants. Under principle 4 of `principles/harness.md`, check for simpler alternatives to the number and format of components, principles, records, and review documents. Focus on problems, misses, gaps, and burdens observed in this work; require a full norms inventory review only when the work shows a need.

## Blocking

A retrospective leaves a human reaction to a stopping point or missed escalation unaddressed, a fresh session cannot answer one of the questions, a blocking condition of a principle named above applies, or a contract named above is violated.

## Exemptions

The exemptions defined in the principles named above apply.
