# Rules and enforcement

For shared decisions, record the decision, its current enforcement mechanism, and grounds. Keep change history in git and PRs; do not keep grounds needed for sharing only in local work records. Record task-specific irreversible items in the plan.

Apply the following rules to the target code and documents. "Enforcement: Not implemented" means that no implementation enforces the rule yet. During retrospective, consider rules marked "Enforcement: Not implemented" and recurring violations for machine enforcement through tests, lint, type checks, pre-commit, or registered verification commands.

## Shared difficult-to-change decisions

None registered. When adding a decision, include its enforcement mechanism and grounds. Work IDs, decision IDs, and task or PR URLs may be added as supplementary references.

## Code structure

- **Do not reuse communication types (requests and responses) internally**. Principle 6 of `principles/structure.md`. Enforcement: Not implemented.
- **Providers and lower layers must not depend on concrete implementations of consumers and upper layers. Dependencies on abstractions (ports) defined by consumers are allowed**. Principle 3 of `principles/structure.md`. Enforcement: Not implemented.
- **Do not import framework, infrastructure, or application configuration types into the domain**. Principles 5 and 6 of `principles/structure.md`. Enforcement: Not implemented.
- **Do not include framework types or annotations in domain types**. Principle 5 of `principles/structure.md`. Enforcement: Not implemented.
- **Do not directly import internal paths without going through a module's public entry point**. Principle 6 of `principles/structure.md`. Enforcement: Not implemented.
- **Do not add functions that only forward arguments unchanged**. Principle 7 of `principles/structure.md`. Enforcement: Not implemented.

## Comments and lint

- **Always provide a nonempty reason for suppressing lint**. Principle 4 of `principles/engineering.md`. Enforcement: Not implemented.

## Tests

A test ID uniquely identifies a test within the repository as the pair of its repository-relative test file path and its identifier in the test framework.

Checking a test's detection ability means deliberately breaking its subject and confirming that the test fails. This differs from detection tests of aspect files (`checks/README.md#detection-tests`).

- **Confirm that tests, including regression tests, fail once before implementation. Record the test ID, target version (`schemas/judgment.json#/$defs/target_version`, the state under verification), and execution result. If this cannot be checked before implementation, use the test detection ability check in the next rule instead**. Principle 4 of `principles/testing.md`. Enforcement: Not implemented.
- **For tests whose failure could not be confirmed before implementation and tests of security-related processing, deliberately break the subject with the smallest necessary diff and record the failure-inducing diff and execution result linked to the test ID**. Principle 4 of `principles/testing.md`. Enforcement: Not implemented.
- **When checking test detection ability, target one property, break it with the smallest necessary diff, run once, immediately revert, and confirm that the source state before the change is restored**. Principle 4 of `principles/testing.md`. Enforcement: Not implemented.
- **Do not leave `skip` or `only` in tests**. Principle 9 of `principles/testing.md`. Enforcement: Not implemented.

## Writing and records

- **Use five confidence values: "Direct", "Supported", "Inference", "Speculation", and "Unknown"**. Principle 2 of `principles/verification.md`. Enforcement: Not implemented.

  - "Direct": Primary information can be quoted directly.
  - "Supported": Multiple pieces of indirect evidence support the same conclusion.
  - "Inference": An interpretation derived from observations.
  - "Speculation": Other explanations are equally plausible.
  - "Unknown": State what was investigated, how, and what was not found.

- **Do not use "obviously", "naturally", or "simply" in prose**. Principle 2 of `principles/verification.md`. Enforcement: Not implemented.
- **Exclude the following machine-readable fixed format from checks requiring conclusions and decision material first**. Principle 1 of `principles/writing.md`. Enforcement: Not implemented.

  - Termination blocks (`schemas/progress.json`, JSON recording round results)

- **Questions requesting human decisions must include options, a recommendation, and its reason**. Principle 7 of `principles/writing.md`. Enforcement: Not implemented.
- **Do not write "I" in PR bodies or review comments**. Principle 2 of `principles/writing.md`. Enforcement: Not implemented.

## Measurement and reporting

- **Measure both sides of a performance comparison under matching build, flag, cache, and data conditions as needed for the scope of the conclusion**. Principle 4 of `principles/verification.md`. Enforcement: Not implemented.
- **Measure both sides multiple times in alternating order (A, B, A, B), and show the median and range**. Principle 4 of `principles/verification.md`. Enforcement: Not implemented.
- **Confirm from output, processed counts, or other evidence that the target processing actually ran during the measurement interval**. Principle 4 of `principles/verification.md`. Enforcement: Not implemented.
- **Use one of "Faster / Slower / No difference / Cannot determine" as the performance conclusion**. Principle 4 of `principles/verification.md`. Enforcement: Not implemented.
- **Performance reports must include the number of runs and variation. Explanations of causes or improvement limits must include the observations needed to support them**. Principle 4 of `principles/verification.md`. Enforcement: Not implemented.

## Screens

- **Do not shift the layout after images or embeds load**. Principle 7 of `principles/ui.md`. Enforcement: Not implemented.
- **Maintain a text-to-background contrast ratio of at least 4.5:1**. Principle 8 of `principles/ui.md`. Enforcement: Not implemented.
- **Do not make necessary information or operations depend only on hover**. Principle 8 of `principles/ui.md`. Enforcement: Not implemented.
- **Use a font size of at least 16px for mobile input fields**. Principle 8 of `principles/ui.md`. Enforcement: Not implemented.
- **Suppress background scrolling only when opening a menu excludes the background from interaction**. Principle 9 of `principles/ui.md`. Enforcement: Not implemented.
- **For animation, check actual responsiveness and rendering quality for unnecessary rendering work**. Principle 9 of `principles/ui.md`. Enforcement: Not implemented.
- **Capture screenshots corresponding to the page, state, and target version (`schemas/judgment.json#/$defs/target_version`) under verification, and retain capture records**. Principle 9 of `principles/ui.md`. Enforcement: Not implemented.
