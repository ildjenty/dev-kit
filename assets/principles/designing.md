# Design approach

## 1. Identify decisions that are hard to change early, and finalize them after obtaining the evidence needed

- **Applicability**: When deciding the order of design decisions or whether implementation, publication, or storage may begin.
- **Exemptions**: When loss is ongoing, waiting to resolve unverified matters would increase the damage, and containment must come first within valid approval or delegated authority. Only decisions necessary for containment may be finalized before resolving unverified matters that affect the choice. Show in the artifact the ongoing loss, the impact of waiting, the response and its effects, remaining unverified matters, and the method and deadline for verification after execution. Do not treat a root-cause fix or permanent design as finalized.
- **Principle**: Identify decisions that are hard to change and the unverified matters needed to make them early. Resolve unverified matters that affect the choice through small investigations or prototypes, and finalize the decisions before starting implementation, publication, or storage that depends on them. Judge reversibility from stored data, running processes, effects on users and callers, and the ability to regenerate and recover, rather than from the kind of target alone. DB schemas, identifiers, types in public contracts, event contents, externally published items, data retention and deletion policies, and file formats are examples to examine. Assess internal contracts from the same perspectives. Decisions confirmed to be changeable later may be made as implementation proceeds. Explicitly state whether to decide now or later as a design decision too.
- **Violations and signs**:
  - Treating a decision that is hard to change as "fixable later" without checking its effects or recoverability.
  - Finalizing a decision while leaving unverified matters that affect the choice unresolved.
  - Deciding reversibility from the kind of target alone.
  - Discussing decisions that can change later first, while starting implementation, publication, or storage dependent on hard-to-change decisions before those decisions are complete.
- **Before and after**: "Investigate later whether deleted data can be retrieved, and start implementing a design that stores no history" → "Use a small prototype to check what information can be retrieved, decide the necessary history retention policy, then start implementation that depends on it."
- **blocking**: A hard-to-change decision is treated as "fixable later" or finalized while unverified matters that affect the choice remain unresolved.
- **Evidence**: A list of hard-to-change decisions, their effects and ability to be regenerated or recovered, investigation and prototype results used for the decisions, finalization deadlines, and who decided each one and when.

## 2. Do not abstract until there is a second concrete example

- **Applicability**: When about to build substitution mechanisms, plugins, or generalizations.
- **Exemptions**: When converting external types to internal types at a boundary under principle 5 of `principles/structure.md` to prevent external types from constraining the internal design. Perform the conversion without waiting for a second concrete example. Do not use this as a reason to create an interface for substitution too.
- **Principle**: Identify the boundary between what changes and what does not, and define only its location and interface. Do not implement substitution mechanisms, plugins, or generalizations until a second concrete example appears. "We may need it in the future" is not a reason to implement it. Keeping a structure that allows implementation when needed is sufficient. Multiple similar pieces of code are preferable to premature abstraction.
- **Violations and signs**:
  - Many interfaces or abstract base classes have only one implementation.
  - Unused optional arguments, configuration fields, or generics exist.
  - Event hooks or plugin mechanisms exist "for future extensibility."
  - Generalization has made the interface unnatural for its only use.
  - (In a design proposal) Substitution or plugin mechanisms are planned without a second concrete example.
- **Before and after**: "Create only `interface Notifier` and `EmailNotifier`" → "Create a `sendEmail()` function, then introduce an interface when another notification destination is needed."
- **blocking**: None (record as reference information).
- **Evidence**: A second concrete example for each abstraction, or the locations and reasons for converting external types at boundaries.

## 3. Verify new connections and changed boundaries early, from the entry point of use to the result

- **Applicability**: When deciding the order of construction for a new feature or connection, or a boundary change.
- **Exemptions**:
  - When attempting to include layers that do not exist in the target or do not affect the inputs, outputs, or conditions needed for the current connection or boundary to work. Waive only the requirement to include those layers in the verification path; do not add or reimplement them for this purpose. Show in the artifact the excluded layers, their relationship to the current inputs, outputs, configuration, authentication, state transitions, and so on, and why connection errors will not be missed. Merely not changing code is not a reason for exemption.
  - When verification through a real connection would cause unauthorized side effects or loss, and no safe verification environment is available. Defer early verification only of that boundary's real connection, and distinguish in the artifact the scope verified through substitutes from the entry point of use and the external contracts the substitutes cannot verify. Also show the conditions for safely using a real connection, verification procedures and deadlines, and the scope that may proceed until then. Do not treat the real connection as verified; use `NOT ASSESSED` if required verification remains.
- **Principle**: Verify new connections and changed boundaries early by running them from the actual entry point of use through to observable results. Exercise small connections and state transitions that determine whether the design works, and find structural errors early. Building out each layer separately causes errors to surface together at final integration. Use the first working feature as an implementation example only for features that share its assumptions. Verify paths with different assumptions separately before extending the approach. Build types and modules used in multiple steps by running them from their first use through to the result too.
- **Violations and signs**:
  - Building out multiple layers or features while the current connection still cannot run.
  - Planning to verify integration of new connections or changed boundaries all at once at the end.
  - Calling a newly built layer "complete" without ever invoking it from a consumer.
  - (In a design proposal) The first step is organized by layer and cannot be verified from the entry point of use to the result.
  - Extending the structure of the first working feature to features with different assumptions without verification.
- **Before and after**: "Complete conversion and output processing separately, then call them from the CLI at the end" → "First read a small input through the CLI and run it through until the converted result reaches standard output, then expand the supported inputs."
- **blocking**: None (the `steps` perspective checks whether each step can be verified independently).
- **Evidence**: How to verify the current connection independently at each step, the entry point of use and observed results, resolved unverified matters, the feature used as an example, and why its assumptions are shared. If any layers were excluded from verification, identify them and the reasons.

## 4. Finish migrations that can safely switch all at once without leaving compatibility layers

- **Applicability**: When planning rewrites, migrations, or API replacements.
- **Exemptions**:
  - When some affected code, running processes, or stored data cannot safely switch at the same time. Use a staged migration only to the extent necessary for compatibility with those targets, and show in the artifact the concrete reasons they cannot switch together, what temporary compatibility covers, and its removal conditions. Do not apply this to parts that can independently and safely migrate all at once.
  - When continued support for an old API or data format is itself an agreed requirement or external contract. Retain compatibility processing needed for that requirement or contract during migration, distinguish it from temporary compatibility, and show in the artifact references to the requirement or contract, covered versions and operations, and whether support termination conditions exist. Do not use this as a reason to retain old implementations unnecessary for compatibility.
- **Principle**: Choose an all-at-once or staged migration based on whether affected code, running processes, and stored data can safely switch all at once. For migrations that can, prioritize the final form over maintaining intermediate states. State the scope in which breaking changes are permitted, and verify the whole at the end of each stage. Introduce the new API, migrate callers, and remove the old API in the same change, leaving no compatibility layer. Maintainability work for temporary code may be limited according to its lifetime, but determine the quality required for correctness, data preservation, and recovery after interruption from the impact of failure.
- **Violations and signs**:
  - A layer left "for compatibility for now" remains after migration.
  - Both old and new APIs remain in use, with no clear source of truth.
  - (In a design proposal) Although a safe all-at-once switch is possible, caller migration and old API removal are split into separate changes, leaving a compatibility layer.
  - Planning an all-at-once switch without checking running old versions or stored state.
  - Omitting processing needed for data preservation or recovery after interruption because the code is temporary.
- **Before and after**: "There are no external users, so remove all old-format readers at once" → "Confirm that old versions have stopped running and how stored state will be migrated and recovered, then replace callers and the old API in the same change within the scope that can safely switch all at once."
- **blocking**: None (record as reference information).
- **Evidence**: Reasons for choosing all-at-once or staged migration, absence of remaining references to the old API, cessation of old-version operation, handling of stored state, and recovery methods. For staged migration, what temporary compatibility covers and its removal conditions.

## 5. Consider removal or replacement before adding a new mechanism

- **Applicability**: When about to add a new mechanism.
- **Exemptions**: None.
- **Principle**: Before adding a new mechanism, consider whether removing or replacing an existing one can meet the requirements. Consider dead code, duplicate validation, stubs, unused options, and other things made unnecessary by the current change or whose removal is required to meet the requirements. Do not add speculative guards or "elegant" boilerplate. Choose the minimum necessary change.
- **Violations and signs**:
  - Not considering removal or replacement of existing mechanisms.
  - Adding a similar mechanism alongside unused branches or options that remain.
- **Before and after**: "Add a new layer for configuration loading" → "Consider replacing the existing loading path, and if it meets the requirements, replace it without adding a new layer."
- **blocking**: None (record as reference information).
- **Evidence**: Removal and replacement candidates considered and their outcomes. For a pure addition, why there is no existing mechanism to replace.

## 6. Review performance, observability, rollout, and the threat model, and choose necessary measures

- **Applicability**: When designing in the `scope` and `structure` stages.
- **Exemptions**: When the emergency containment exemption in principle 1 of `principles/designing.md` applies and waiting to resolve unverified matters about performance, observability, rollout, or the threat model would increase damage. Only for decisions necessary for containment, waive the requirement to resolve and verify these matters before finalization; do not omit reviewing the four perspectives or considering their effects, and do not apply this to finalizing permanent designs. Show in the artifact the damage that would increase while waiting, the approval or delegation scope, decisions made in advance, remaining unverified matters, and the method and deadline for verification after execution.
- **Principle**: Review performance (volume handled and latency), observability (how to check results and failures), rollout (how to apply and roll back), and the threat model (what to protect from whom) in the same stage as structure. Distinguish reviewing these perspectives from introducing dedicated mechanisms, and choose measures according to usage conditions and the impact of failure. Use existing mechanisms or operations if they suffice; introduce dedicated mechanisms when the shortfall can be explained. Defer only matters that do not determine whether the current design works or for which a verification procedure before finalization has been defined. State what may proceed while decisions remain open, and do not finalize dependent structures or publication while unable to determine the necessary guarantees.
- **Violations and signs**:
  - Saying "Performance can wait" or "We will decide how to detect failures during implementation" without checking usage conditions or the impact of failure.
  - Introducing dedicated mechanisms when existing mechanisms or operations suffice.
  - Proceeding with dependent design without a method and deadline for resolving the required guarantees.
- **Before and after**: "Consider input volume, failure detection, distribution, and what to protect after implementation" → "Try expected inputs in the local CLI and inspect failure reasons on standard error. Distribute by replacing the executable, and process inputs locally without overwriting input files."
- **blocking**: Matters that determine whether agreed requirements or safety can be met are deferred without a resolution method or deadline.
- **Evidence**: For the four perspectives, how usage conditions, requirements, and failure impacts correspond to measures; why existing mechanisms or operations suffice; and whether dedicated mechanisms are necessary. For deferred matters, why they do not determine whether the current design works, or the procedure and deadline for verification before finalization, and the scope that may proceed while undecided.

## 7. Follow existing conventions

- **Applicability**: When adding new code or designs to an existing repository.
- **Exemptions**: When existing conventions do not meet agreed requirements or necessary guarantees, and following them would compromise those requirements or guarantees. Waive the requirement to follow existing conventions only for the differences necessary to meet those requirements or guarantees. Show in the artifact references to the requirements and guarantees, existing locations compared, concrete conditions that cannot be met, and evidence that another approach meets them. Follow existing conventions for other naming, error formats, log formats, and so on.
- **Principle**: Before writing, find how the repository handles the same kind of processing (naming, error-handling forms, layer boundaries, existing utilities and abstractions, test style, and libraries), and follow conventions that meet the current requirements and guarantees. When adopting a different approach, migrate only the scope inseparable from the current change. If making everything consistent is an independent improvement, make it a separate issue. Show in the artifact the existing locations followed, or the locations not followed, differences, and reasons.
- **Violations and signs**:
  - The same kind of processing exists in two styles.
  - Adding a new helper with the same role despite an existing utility or abstraction.
  - Error handling or log formats differ from surrounding code.
  - Introducing a new library alongside an existing library with the same role.
  - (In a design proposal) Neither existing locations to follow nor reasons for not following them are shown.
- **Before and after**: "Returned new API errors using a custom exception class" → "Examined the `Result` type used by existing APIs and returned the same form. No exception class is added."
- **blocking**: The style differs from existing processing of the same kind without a stated reason.
- **Evidence**: References to existing locations followed, or locations not followed, differences, and reasons. For migrations, why the scope is inseparable from the current change.
