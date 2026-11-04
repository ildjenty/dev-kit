# Data and state

## 1. Define the source of truth for facts with the same subject, time, and meaning

- **Applicability**: When considering keeping the same information in multiple places. When considering sharing similar processing.
- **Exemptions**: When storing derived values or duplicating data is necessary to meet performance requirements, waive only the requirement to calculate derived values instead of storing them. Show the relevant performance requirements and evidence that storage or duplication is necessary. Also show the source of truth, who owns synchronization, the update method, the permitted inconsistency and its scope, and what to do if it exceeds that scope.
- **Principle**: Define one source of truth for facts with the same subject, time, and meaning. Calculate derived values that can be reproduced from the current source of truth instead of storing them. Store values needed to reproduce decisions or transactions at a past point in time as facts from that time. For duplicates, define the source of truth, who owns synchronization, the update method, the permitted inconsistency, and what to do if it exceeds that scope. Processing that must always reflect changes to the same rule simultaneously must share the rule's definition; keep processing independent when its reasons for change or contracts differ, even if its text is identical.
- **Violations and signs**:
  - Data representing facts with the same subject, time, and meaning exists in multiple places and is updated through separate paths.
  - Boolean state flags keep increasing (when a single state transition could actually express them).
  - Deciding which value is correct at runtime by majority vote or timestamp comparison.
  - Combining separate rules in one place merely because they happen to have the same shape (they will need separating again when only one must change).
  - The update method or permitted inconsistency of a duplicated list is undefined.
  - (In a design proposal) The same fact is stored in multiple places, but the source of truth and synchronization owner are not stated.
  - Multiple modules independently define format or ordering rules that must change together.
  - Multiple processing paths must apply the same rule, but there is no guarantee that changes are reflected simultaneously.
- **Before and after**: Store both `user.isPremium` and `subscription.plan` → Calculate `isPremium` from `subscription.plan`. Recalculate an order amount from current product unit prices → Store unit prices at the time of the order and reproduce the transaction at that time.
- **blocking**: Facts with the same subject, time, and meaning are updated independently in multiple places without a defined source of truth and synchronization method.
- **Evidence**: The subject, time, and meaning of facts, and the location of their source of truth. The synchronization owner, update method, and permitted freshness or inconsistency of duplicates. Processing that shares the same rule, and processing kept independent because its reasons for change or contracts differ.

## 2. Define states and responses for failure, duplication, and interruption in advance

- **Applicability**: When designing external communication, message processing, or processing that may stop partway through.
- **Exemptions**: When failure, duplication, or interruption leaves no external or internal state requiring recovery other than fetching again, and failure notification and fetching again satisfy the contract, only the recovery procedure may be omitted. Show how intermediate results, resources in use, and duplicate results are handled. Also show how failure is reported and why fetching again is safe.
- **Principle**: Treat external service outages, duplicate messages, and processing interruptions as normal inputs. Define in advance the states left by failure, duplication, or interruption and the acceptable outcomes, then choose the necessary responses from aborting, discarding, recalculating, rerunning, and manual verification. For processing that leaves external side effects, define recovery according to the external system's ability and guarantees to identify and query the operation, including cases where the outcome becomes unknown.
- **Violations and signs**:
  - Failure behavior is undefined because "it probably will not happen."
  - Adding retries and timeouts whenever a problem occurs.
  - Waiting without a timeout (external calls, locks, or queues).
  - Stopping partway through leaves an unacceptable state.
  - For processing that leaves external side effects, failing to check whether the outcome can be identified and queried when a response is lost.
  - Rerunning while the external outcome remains unknown, potentially duplicating side effects.
  - (In a design proposal) The state left when external communication stops partway through and the recovery method are not stated.
- **Before and after**: Rerun a payment immediately if its response is lost → Check the external system's identification and query guarantees; resume from the query result if the outcome can be established, and otherwise send it for manual verification without rerunning.
- **blocking**: The states left by failure, duplication, or interruption, acceptable outcomes, or necessary responses are undefined.
- **Evidence**: The state left at each possible point of failure, duplication, or interruption, acceptable outcomes, and responses. The external system's identification and query capabilities and guarantees, handling of unknown outcomes, and the scope that cannot be recovered.

## 3. Prevent invalid states and values with types and validation

- **Applicability**: When deciding on types, data structures, or interfaces.
- **Exemptions**:
  - When DB constraints or other means enforce the same guarantee on every acceptance, storage, and output path that must reject the invalid state, and adding the constraint to types would duplicate management of the same rule. Waive only the requirement to add a type for that property; validation and conversion of external input and the guarantee of rejecting invalid states remain necessary. Show in the artifact the invalid state, the guarantee mechanism and the boundary enforcing it, the covered paths, and why they cannot bypass it.
  - When separating values into types by kind would prevent no possible mix-up. Waive only the requirement to introduce that type; validation and conversion of external input and the guarantee of rejecting invalid states remain necessary. Show in the artifact the values, their uses, and why mix-ups cannot occur.
- **Principle**: Prevent invalid combinations of states through types to the extent naturally expressible in the type system being used, and represent failure kinds with types too. Guarantee constraints dependent on values or runtime state through boundary validation, DB constraints, and checks during updates. Define guarantee mechanisms for storage formats and external contracts too; do not rely only on care or review. When mixing up values of the same primitive type causes concrete misbehavior that distinct types can prevent, use types that distinguish identifiers, units, validated values, and so on. Do not treat external data as a validated type until it has been validated and converted.
- **Violations and signs**:
  - Values that should be mutually exclusive are both listed as optional fields.
  - A value meaningful only in a particular state is separate from that state (the type does not know that an approver exists only when approved).
  - Failures return `null` or `boolean`, leaving callers unable to distinguish what happened.
  - Calling conventions such as "Call only after..." exist only in comments.
  - Receiving a known type as `unknown` or obscuring it with casts.
  - Writing a single incorrect value grants stronger privileges.
  - (In a design proposal) Mutually exclusive states or values can be represented, but no validation or constraint rejects them.
- **Before and after**: `{ status: string; approvedBy?: string }` → `{ status: "draft" } | { status: "approved"; approvedBy: UserId }`
- **blocking**: Invalid states are accepted, stored, or output without guarantees preventing them.
- **Evidence**: How types, validation, and constraints correspond to invalid states or concrete mix-ups to prevent. Verification that invalid values or updates cause type errors or are rejected by boundary validation, DB constraints, or checks during updates.

## 4. Express domain rules in one structure rather than scattered branches

- **Applicability**: When about to add a conditional branch. When designing around states or kinds.
- **Exemptions**: Even when rules branch on the same key, if they change independently and combining them would propagate unrelated changes, waive only the requirement to combine those rules in one structure. Apply this principle within each rule. Show each rule's reasons for change and a concrete example where only one changes.
- **Principle**: Express domain rules with structures (state machines, tables, sum types, or reducers) rather than scattered conditional branches. Choose structures that prevent missed rule updates and invalid transitions. Do not combine independently changing rules (such as pricing and backup retention periods) into one structure merely because they branch on the same kind.
- **Violations and signs**:
  - `if / else` branches concerning the same rule keep growing in multiple places.
  - A second boolean appears that must be synchronized.
  - The same rule is independently defined in multiple places.
  - (In a design proposal) Rules for states or kinds that must change together are written as conditional branches across multiple modules rather than expressed in a shared structure.
- **Before and after**: Define the same pricing rule in multiple conditional branches → Share a pricing table, and separately define the independently changing backup retention rule.
- **blocking**: None (record as reference information).
- **Evidence**: The structure prevents missed rule updates and invalid transitions.

## 5. Define data ownership and lifetime, and the boundaries that protect invariants

- **Applicability**: When deciding or changing data structures, update paths, ownership, or lifetimes.
- **Exemptions**: None.
- **Principle**: Define entity boundaries, data ownership, and lifetimes, and list the invariants that must hold. Define which boundary protects each invariant against interruption and contention. For updates across boundaries, define acceptable temporary states and recovery methods.
- **Violations and signs**:
  - Data has no defined owner.
  - State has no defined lifetime.
  - Transaction boundaries are implicit.
  - An interruption leaves a parent record without required detail records.
  - Acceptable temporary states or recovery methods are undefined for updates across boundaries.
- **Before and after**: Save an order and its details separately without defining interruption handling → The order owns its details, which have the same lifetime as the order. Protect the invariant "A confirmed order has its required details" by saving the order and details in the same transaction.
- **blocking**: Updates exist without defined boundaries guaranteeing the necessary consistency.
- **Evidence**: How entity boundaries, ownership, lifetimes, and invariants correspond to guarantee boundaries and mechanisms. Conditions remain protected during interruption and contention, and acceptable temporary states and recovery methods are defined for updates across boundaries.

## 6. Reduce sharing for independent updates, and preserve shared invariants during concurrent updates and integration

- **Applicability**: When designing multiple processes (parallel tasks, agents, or workers) that update the same state or affect shared invariants.
- **Exemptions**: None.
- **Principle**: Reduce shared mutable state for independent updates. When updates affect the same invariant, do not assume separate storage locations alone guarantee it; show that concurrent updates and integration of results preserve it too. If update and integration rules alone cannot guarantee it, choose the simplest coordination method that provides the necessary guarantees, such as a single writer, transactions, conditional updates, or locks. A convention to "write carefully" cannot control concurrency.
- **Violations and signs**:
  - Multiple writers affect the same invariant, but there is no evidence that concurrent updates and integration of results preserve it.
  - A warning says "Do not run concurrently."
  - Assuming that separating storage locations also protects shared invariants such as total inventory.
  - (In a design proposal) Processing affects the same invariant despite separate storage locations, but no guarantee of preserving it through concurrent updates and integration of results is stated.
- **Before and after**: Write to separate sales records, allowing sales beyond available inventory → Preserve consistency between inventory and sales records through transactions and conditional updates that allow sales only when inventory is sufficient.
- **blocking**: Invariants cannot be guaranteed under contention.
- **Evidence**: Each writer's storage location, shared invariants, permitted updates, integration rules, and why they preserve the invariants. If coordination is necessary, its method and why it provides the required guarantees.

## 7. Preserve required meaning in data representations and conversions

- **Applicability**: When deciding value storage formats, units, precision, normalization, or conversion methods.
- **Exemptions**: None.
- **Principle**: First define the distinctions and precision needed by subsequent processing, then choose a representation that preserves them. When rounding or normalization loses information, define why that loss is acceptable and when to perform it. Retain original values if subsequent processing or recalculation needs them.
- **Violations and signs**:
  - Calculating without distinguishing currencies or units.
  - Rounding on import loses precision needed for subsequent calculations.
  - Overwriting original values needed for display or reprocessing with normalized search values.
- **Before and after**: Round and store amounts on import → Store them at the required original precision, then round at the stage defined for aggregation or finalization.
- **blocking**: Representation or conversion loses required distinctions, precision, or the ability to recalculate.
- **Evidence**: Required distinctions and precision, information lost in conversion and why its loss is acceptable, and the timing of conversion. Verification results showing that representative and boundary values meet the requirements.
