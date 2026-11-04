# principles

The repository owns and develops the principles introduced by `devkit init`. Checks select these principles and the review documents in `principles/review/` as their judgment criteria. Do not turn rules that machines can judge deterministically into principles. Address matters concerning the loop mechanism in the mechanism's definitions; put rules concerning the target code and documents, along with their enforcement mechanisms, in `rules.md`. Leave mechanisms marked as "not implemented" until they are implemented.

- **`principles/engineering.md`**: When writing code
- **`principles/designing.md`**: When deciding how to approach design
- **`principles/structure.md`**: When deciding module placement and dependencies
- **`principles/data.md`**: When deciding on data and state
- **`principles/testing.md`**: When writing tests and before changing existing code
- **`principles/verification.md`**: When making statements about tools, APIs, versions, causes, or measurements
- **`principles/working.md`**: When deciding how to work
- **`principles/context.md`**: When handling context and records
- **`principles/harness.md`**: When reviewing the loop mechanism (parts, assembly, and plans) in a retrospective, or running a check that selects `principles/review/harness.md`
- **`principles/ui.md`**: When designing or building screens
- **`principles/writing.md`**: When writing text for people to read (reports, PR descriptions, and documents)

## Principle format

Describe each principle using the following fields.

- **Name**: A short name that can be referenced in conversation. People can give instructions by name, such as "Review this against 'Do not abstract until there is a second concrete example.'"
- **Applicability**: State when the principle applies in one sentence beginning with "When."
- **Exemptions**: Conditionally exempt requirements of this principle that would make things worse if followed. State the "when" condition, the scope of the requirements being waived, and what the artifact must show to establish that the condition applies. If there are multiple conditions, use nested bullets for each condition. Even when an exemption applies, only the waived requirements cease to count as violations. The principle's other requirements and blocking conditions remain in force. Do not list ordinary actions already permitted by the body as exemptions. If none come to mind, write "None"; "None" is subject to criticism seeking counterexamples.
- **Principle**: What to do and why.
- **Violations and signs**: Describe these so that a diff or design proposal can be examined to determine whether they apply.
- **Before and after**: Give one pair of before and after examples.
- **blocking**: Conditions under which a check makes a violation of this principle blocking.
- **Evidence**: Evidence to present to establish that the principle has been met.

In responses, agents must state which principle changed which decision. Merely naming a principle without changing a decision does not count as applying it. When using an exemption to waive a principle's requirements, also state which exemption applies, the scope of the waived requirements, and the reason.

## When principles conflict

- **Principle 2 of `principles/designing.md`, "Do not abstract until there is a second concrete example," and principle 5 of `principles/structure.md`, "Keep framework and external service types out of public contracts, stored data, and domain types"**: Follow principle 5 of `principles/structure.md` as an exemption to principle 2 of `principles/designing.md`. Reason and tradeoff: Perform conversions that isolate public contracts, stored data, and domain types from external specifics without waiting for a second concrete example. Do not force conversions when an exemption to principle 5 of `principles/structure.md` applies, such as value types from general-purpose libraries. The tradeoff is the cost of conversion and maintenance; show whether it is justified by the knowledge being hidden and the reach of changes. Do not go as far as creating an interface for substitution
- **Principle 1 of `principles/data.md`, "Define the source of truth for facts with the same subject, time, and meaning," and principle 6 of `principles/structure.md`, "Hide decisions that may change inside modules"**: Priority: principle 1 of `principles/data.md`. Reason and tradeoff: Preserve facts from past points in time. When duplicating data for performance, define the source of truth, who owns synchronization, the update method, the permitted inconsistency, and what to do if it exceeds that scope. Principle 6 of `principles/structure.md` defines what is exposed; hide storage methods in the module responsible for them
- **Principle 4 of `principles/structure.md`, "Make guarantees required on every path impossible to bypass," and principle 3 of `principles/structure.md`, "Keep lower-layer contracts and implementations independent of specific consumers"**: Priority: principle 3 of `principles/structure.md`. Reason and tradeoff: Write cross-cutting processing without knowing consumers' names (general-purpose middleware). If it needs branches for individual consumers, it belongs to those consumers rather than at a shared passage point
- **Principle 5 of `principles/designing.md`, "Consider removal or replacement before adding a new mechanism," and principle 7 of `principles/working.md`, "Do not make unrequested changes"**: Priority: principle 7 of `principles/working.md`. Reason and tradeoff: Deletion is also a change. Consider things made unnecessary by the current change or whose removal is required to meet the requirements; do not mix in unrelated cleanup. If deletion is outside the request's scope, report it as a proposal
- **Principle 7 of `principles/designing.md`, "Follow existing conventions," and principle 4 of `principles/designing.md`, "Finish migrations that can safely switch all at once without leaving compatibility layers"**: Priority: principle 4 of `principles/designing.md` for migrations within the scope where a change to existing conventions has been decided; principle 7 of `principles/designing.md` when following existing conventions that meet the current requirements and guarantees. Reason and tradeoff: Limit migration to what is inseparable from the current change, and leave no compatibility layer if a safe switch all at once is possible. If a staged migration is necessary, define what temporary compatibility covers and the conditions for its removal. If making everything consistent is an independent improvement, make it a separate issue

## How to update principles

Add or change only a small number of principles at a time. Do not record individual incidents verbatim; express them in a form that applies to other situations. If a principle overlaps with an existing one, add to the existing entry instead of creating another. In the retrospective in `stages/retrospective.md`, use the problems, oversights, and gaps or burdens in norms encountered during the work to check which checks and templates select each principle, whether any principles are unused, and whether any cost more than they are worth; consider organizing and reducing them. Review the entire set of norms only when a need becomes apparent. Treat changes to principles as changes to the loop itself and, by default, obtain human review before they first take effect.
