# Modules and dependencies

## 1. Make roles clear through names and placement

- **Applicability**: When creating modules, files, or folders, or deciding where to place them.
- **Exemptions**: When generation rules or external specifications fix names or placement, and changing them would break regeneration or correspondence with the specifications, exempt only those fixed names and placements. Do not exempt separation of responsibilities or access to contracts, constraints, and design reasons. Show in the artifact the fixed items and references to the rules or specifications requiring them.
- **Principle**: Structure modules so that names, placement, and dependencies show what each module does and why it belongs there. Preserve contracts, constraints, and design reasons that structure alone cannot express in a form accessible from where they are needed. Separate processing with independent reasons for change, and group knowledge that must always be changed and understood together. Follow the repository's "placement guide" (what belongs and does not belong, documented in each package's README or CLAUDE.md). Check on the assumption that the next developer is an agent that references only files it opens.
- **Violations and signs**:
  - Mixing processing with independent reasons for change. Or scattering knowledge that must always be changed and understood together.
  - Code has no clear place to belong and is placed wherever convenient.
  - Contracts, constraints, or design reasons needed for decisions cannot be referenced from the relevant code.
  - (In a design proposal) A new module's placement cannot be justified by its responsibilities.
- **Before and after**: `formatPrice` and `retryFetch` in `utils/helpers.ts` → `pricing/format.ts` and `http/retry.ts`
- **blocking**: Placement cannot be justified by responsibilities.
- **Evidence**: Each new module's role and placement rationale, and access to contracts, constraints, and design reasons needed to understand it. For splits, show processing that can change independently and knowledge that must remain shared afterward.

## 2. Do not repeat validation while its validity conditions remain true

- **Applicability**: When designing processing that receives external values (user input, APIs, files, or environment variables), or processing that validates the same conditions repeatedly.
- **Exemptions**: None.
- **Principle**: First define the boundaries between external and internal, and untrusted and trusted data; validate, convert, and isolate at those boundaries. Do not repeat validation while the same validity conditions remain true; internally, rely on properties guaranteed by internal types. Distinguish type guarantees from guarantees dependent on runtime state, and check conditions dependent on the actor, target, authority, time, or mutable state at the boundary of the operation that requires them. Being validated does not establish safe use in commands, paths, or SQL. Separate values from instructions according to how the destination interprets them, and limit targets to the permitted scope.
- **Violations and signs**:
  - Repeating the same null or type checks at the start of many functions despite preserved validity conditions.
  - Passing potentially missing values deep into internal processing.
  - Handling validation errors differently in different places.
  - Skipping runtime authority or target-state checks because input was validated at intake.
  - Incorporating validated external input into commands, paths, or SQL without separating instructions or restricting target scope.
  - (In a design proposal) The external input validation boundary and conversion to internal types are not stated.
- **Before and after**: Both `handler` and `service` validate the same ID format → Convert with `parsePlanRequest(req.body): PlanRequest` in `handler`; `service` does not revalidate the format, and checks authority and target state at the update operation's boundary
- **blocking**: External input can be interpreted as instructions or as a target outside the permitted scope.
- **Evidence**: The conditions under which guarantees hold and where they are maintained or rechecked. Show how separation of values from instructions and restrictions on target scope are preserved where commands, paths, or SQL are used.

## 3. Keep lower-layer contracts and implementations independent of specific consumers

- **Applicability**: When deciding import direction between modules or layer boundaries.
- **Exemptions**: When comments, errors, or logs name a consumer as a usage example or reference necessary to understand the contract, exempt only that reference, and only if the wording containing the name does not implement branching, compatibility, or consumer-specific constraints. Show in the artifact why the reference is necessary and why changing or removing the consumer does not require changing lower-layer contracts, behavior, or decisions.
- **Principle**: Define abstractions (ports) on the consumer side, not the implementation side. Keep lower-layer contracts and implementations independent of specific consumers' circumstances. Fix dependency problems by reviewing responsibilities and boundaries rather than using workarounds. Evaluate dependencies in comment, error, and log wording by whether consumer changes require changes to lower-layer contracts, behavior, or decisions.
- **Violations and signs**:
  - Lower-layer comments treat a specific consumer's circumstances as lower-layer contracts or constraints.
  - Shared parts contain specific consumers' circumstances (flags or branches).
  - Avoiding cycles with re-exports or lazy imports.
  - A lower layer has helpers for an upper layer merely for convenience.
  - (In a design proposal) Consumer-specific changes also require changes to lower-layer contracts, behavior, or decisions.
- **Before and after**: `db/users.ts` branches on a `features/billing` flag → `features/billing` calls `db/users`, and branching belongs in `billing`
- **blocking**: Lower-layer responsibilities or explanations depend on specific consumers' circumstances.
- **Evidence**: Each layer's responsibilities and the conditions under which consumer changes propagate to lower layers. If retaining usage examples or references, show why they are necessary to understand the contract.

## 4. Make guarantees required on every path impossible to bypass

- **Applicability**: When placing processing intended to apply on every path, such as validation, isolation, recording, measurement, or authorization.
- **Exemptions**: When guarantees are enforced per boundary or target, such as through DB policies, and completeness of target coverage and reasons they cannot be bypassed can be shown.
- **Principle**: Enforce guarantees required on every path through structures that prevent omissions. Consolidate them at a shared passage point if possible. Regardless of placement, eliminate paths that can bypass required guarantees instead of relying solely on conventions or reviews to request application.
- **Violations and signs**:
  - Relying on conventions or reviews saying "Do not forget the same check in this processing too."
  - Copies of the same processing differ slightly by feature.
  - Each new path requires manually attaching the processing, and omissions cannot be detected.
  - Writing authorization checks separately for each path leaves a path that allows operations without a check.
  - (In a design proposal) Entry points or targets are not listed, and there is no evidence that required guarantees can be enforced on all of them.
- **Before and after**: `assertTenant(req)` at the start of each handler → Enforce it in router middleware through which every path to each handler must pass
- **blocking**: A path can bypass required guarantees.
- **Evidence**: A list of entry points, boundaries enforcing guarantees, and checks for omissions. When enforcing per boundary or target, show completeness of target coverage and why bypass is impossible.

## 5. Keep framework and external service types out of public contracts, stored data, and domain types

- **Applicability**: When deciding public contracts, stored data, or domain types in processing that uses frameworks or external services.
- **Exemptions**:
  - At boundaries directly operating external mechanisms, such as framework handler contracts, when hiding external types would add abstractions that merely relay information. Waive only the ban on external types in that boundary's contract; propagation into consumers, stored data, or domain types that do not need to handle them directly is not exempt. Show in the artifact the boundary's responsibilities, operations requiring external types, and the scope to which types propagate.
  - When using value types from general-purpose libraries, such as numbers or dates and times. Waive only the ban on using that value type, on condition that storage and communication formats and restoration methods are separately defined when used for those purposes; using the internal representation as the storage contract is not exempt. Show in the artifact the value type's meaning, usage scope, storage or communication formats, and restoration methods.
  - When no substitution is planned, conversion would hide no external-specific knowledge, the entire scope using external types can change together, and it includes no public contracts or storage formats whose compatibility must be maintained independently. Waive only the ban on external types within that scope. Show in the artifact why conversion cannot hide knowledge, the reach of external-type changes, and why the scope can change together.
- **Principle**: Keep framework and external service types out of public contracts, stored data, and domain types. At boundaries, convert them to types suited to the responsibilities there, and do not expose external-specific names, constraints, or lifecycles to consumers. Establish conversion layers based on knowledge that currently needs hiding and the reach of changes; possible future substitution alone does not justify adding them.
- **Violations and signs**:
  - Public contracts, stored data, or domain types change to suit frameworks or external services.
  - Adding custom types and conversions that merely relay the same information solely for future substitution.
  - (In a design proposal) Public contracts, stored data, or domain types include framework or external service types.
- **Before and after**: `function charge(customer: PaymentService.Customer)` → `function charge(customer: Customer)` with `toCustomer(externalCustomer)` at the boundary
- **blocking**: None (record as reference information).
- **Evidence**: External-specific knowledge hidden by conversion and the reach of changes into public contracts, stored data, and the domain. Show why the conversion and maintenance burden is justified, or why conversion is not performed.

## 6. Hide decisions that may change inside modules

- **Applicability**: When deciding what a module exposes (public functions and types).
- **Exemptions**: When meeting agreed requirements or an existing compatibility contract that must be preserved requires exposing the data format or processing method itself, and exposing results or operations alone is insufficient, exempt only the elements that must be exposed. Show in the artifact references to the requirements or compatibility contracts, necessary operations and verification, and why hiding the format or method would fail to meet those requirements or contracts. Dependencies of consumers created in the current work alone do not establish that exposure is necessary.
- **Principle**: Hide decisions that may change later, such as data storage methods, tools used, or processing methods, inside the module responsible for them. Expose only contracts that change infrequently. Minimize what is exposed, because other modules use it and make it hard to change. Hiding internally does not mean adding abstractions (ports) (principle 2 of `principles/designing.md`).
- **Violations and signs**:
  - Internal representations (table columns or external API response shapes) pass directly to other modules.
  - Functions or types unused anywhere are public.
  - Changing one module's internals requires changing consumers too.
  - (In a design proposal) The contract directly exposes data storage methods or external API response shapes.
  - A module is shallow: its interface is as large as its implementation, hiding nothing.
  - Requirements and methods are listed as the same kind of thing, mixing "what must be met" and "how to achieve it" in one list, making decisions to change methods indistinguishable from decisions to preserve requirements.
- **Before and after**: `export const db` → `export function findPlan(id)`
- **blocking**: None (record as reference information).
- **Evidence**: A list of what is exposed and why each item is a promise.

## 7. Reduce layers to traverse and state to remember

- **Applicability**: When adding layers. When evaluating the effects of refactoring.
- **Exemptions**: When removing a relay layer would lose necessary contracts, knowledge hiding, or constraint enforcement, and no simpler structure preserving them can reduce the burden of change, understanding, or verification, exempt only retaining that layer. Show in the artifact the layer's contracts, hidden knowledge, enforced constraints, and comparison with removing it.
- **Principle**: Use "number of layers to traverse" and "amount of state to keep in mind" as indicators of the burden of understanding. Remove layers that merely relay information without expressing contracts, hiding knowledge, or enforcing constraints. Reduce state scope (pure functions > local > field > module > global). Line count and cyclomatic complexity are only proxy measures. Evaluate refactoring by whether it reduces the burden of change, understanding, and verification while preserving necessary guarantees.
- **Violations and signs**:
  - Tracing processing passes through a layer responsible for no contracts, knowledge, or constraints.
  - Adding a layer solely to avoid changing existing code.
  - (In a design proposal) Adding mechanisms to satisfy constraints without showing what knowledge they hide or guarantees they enforce.
- **Before and after**: `Controller → Service → Manager → Repository → db` (Service and Manager only relay information and carry no contracts, knowledge, or constraints) → `Controller → Repository → db`
- **blocking**: A simpler structure can meet the same purpose while preserving boundary guarantees.
- **Evidence**: Changes in layers and state before and after, knowledge exposed by removal, and why boundary guarantees can be preserved. Compare concrete situations of change, understanding, and verification to show reduced burden.
