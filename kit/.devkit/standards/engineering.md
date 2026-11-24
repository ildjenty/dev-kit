# Principles for writing code

## 1. Do not hide failures

- **Applicability**: When deciding behavior for errors, emptiness, zero results, exceeded limits, missing values, or duplicates, or deciding how to record causes internally and respond publicly.
- **Exemptions**: None.
- **Principle**: Distinguish normal absence, permitted degradation, and failure according to the operation's contract. Do not substitute normal absence or default values for failure. Continue after a failure only if the promised successful outcome is met and the failed portion can be reported and recorded according to the contract. Reject operations, such as authorization, that cannot be permitted when a decision cannot be made (fail-closed). Preserve causes needed for recovery, investigation, and control in a form that authorized internal processing can distinguish. Determine public responses from what may be disclosed and what callers need to do. Even when different causes share the same public error for confidentiality, preserve their internal distinction.
- **Violations and signs**:
  - Using `catch` to log an error and return success without meeting the promised outcome.
  - Using `catch { return null }` or `catch { return [] }` to turn failure into "there was no data."
  - Handling an empty identifier with a default value and mixing data that should remain separate.
  - Branching only on whether a value exists without defining what its absence means.
  - Losing internal distinctions among causes needed for recovery, investigation, or control when combining public responses.
  - Allowing an operation that should be rejected when validation or authorization cannot be decided.
  - Not retaining information needed to investigate causes. Or leaving confidential information in logs or errors.
  - Not releasing allocated resources (files, connections, timers, or subscriptions) on failure.
  - Adding retries to everything (including non-idempotent operations and failures that rerunning cannot fix).
  - No handler awaits asynchronous failures, so failures go undetected.
  - Allowing external writes or processing while contractually required configuration or prerequisites remain uncertain.
- **Before and after**:
  ```ts
  // before
  try { return await fetchPlan(id) } catch { return null }   // Failure becomes "no plan exists"
  // after
  try { return await fetchPlan(id) }
  catch (e) { throw new PlanFetchError(id, { cause: e }) }   // Distinguish fetch failure from normal absence
  ```
- **blocking**: The diff contains a path that treats the outcome as success without meeting the scope promised as success in the evidence, or continues without rejecting when the contract requires rejection, including when validation or authorization cannot be decided.
- **Evidence**: The scope promised as success and why continuation after failure is possible. Show who notices each failure path and where, that necessary causes can be investigated internally, and that public responses do not leak confidential information.

## 2. Prevent duplicate side effects on reruns and preserve invariants during concurrent execution

- **Applicability**: When writing paths with redelivery, retries, or reruns after timeout, or processing that handles the same data concurrently.
- **Exemptions**: Only when the contract permits duplicate observation events, including their effects on subsequent aggregation and decisions, and preventing duplication would prevent the main processing from meeting its requirements, waive duplicate prevention for observational side effects caused by that recording or transmission. Show in the artifact the permitted scope of duplication, its effects on subsequent processing, and the requirements that duplicate prevention would make impossible to meet, with supporting evidence. This does not apply to operations such as billing or authorization, or to concurrency invariants.
- **Principle**: On reruns, identify the same operation using the identification method defined by its contract, and do not duplicate side effects. Execute distinct operations even if their input values are identical. If the same operation cannot be identified, do not treat rerun safety as guaranteed. Even with the same identifier, explicitly distinguish inputs that determine the operation's meaning but do not match existing records from normal reruns. Preserve contractually defined invariants when distinct, conflicting operations execute concurrently. Perform checks and updates within the same boundary that guarantees consistency, and permit order-dependent results only as allowed by the contract. Choose boundaries under principles 5 and 6 of `data.md`, and respect them in implementation. Make locks, temporary files, and incomplete records left by interruption identifiable as stale on the next execution (write to a temporary file and then rename, or compare contents).
- **Violations and signs**:
  - Unconditionally registering or sending redeliveries of the same operation, duplicating side effects.
  - Treating identical input values alone as duplication and not executing legitimate distinct operations.
  - Ignoring inputs with different meanings under the same identifier as already processed.
  - Calculating times on a rerunnable path on the assumption that execution will not be duplicated.
  - A sequence of updates leaves an incomplete state if stopped partway through (no transaction or rollback).
  - Another process can intervene between reading and checking and then writing (checks and updates are separate).
- **Before and after**: "Check the balance, debit it separately, and ignore duplicate identifiers" → "Check the operation identifier and inputs within the same consistency boundary; do not debit again for the same operation, and reject mismatched inputs. If unprocessed, check the balance, debit it, and record completion."
- **blocking**: Rerunning the same operation duplicates side effects, inputs with different meanings under the same identifier are treated as normal reruns, or concurrent conflicting operations violate invariants.
- **Evidence**: The contractually defined method for identifying the same operation. Show that rerunning the same operation does not duplicate side effects, distinct operations with identical contents execute, and inputs with different meanings under the same identifier are distinguished. Demonstrate through tests or execution that invariants hold even when conflicting processing attempts to intervene between checks and updates.

## 3. Choose names that first-time readers can understand

- **Applicability**: When naming variables, functions, types, files, or folders.
- **Exemptions**:
  - When a name is part of an external contract or stored data and changing it would break existing calls or reads. Waive only naming requirements that prevent retaining contractually necessary names. Show in the artifact the retained names and the contracts their change would break.
  - When user-facing error messages must use the language required for the product, or existing verification confirmed as requiring preservation treats non-English text as a contract and changing it would violate that contract. Waive only the requirement to write that text in English. Show in the artifact the retained text, the contract its change would break, and the basis for the language requirement or the need to preserve the verification.
- **Principle**: Use terms from the code and documents; do not introduce terms invented in conversation. Use consistent names for the same things, and change names when their contents change. Names must express the scope of their subjects' responsibilities. Names that are too narrow cease to match when responsibilities expand. Names that are too broad allow responsibilities to expand without limit. Write test names and error messages in English to standardize search terms when investigating logs and errors.
- **Violations and signs**:
  - Names whose responsibilities are unclear even with their placement (`data` `info` `result` `item` `temp` `helper` `util` `manager` `handler`).
  - Inconsistent names for the same thing (customer / client / user all refer to the same thing).
  - Boolean names that do not match reality (`isValid` actually means "has been saved").
  - Names chosen only for their difference from existing names, such as `processData2` or `newHelper`.
  - Naming processing or data that covers the whole after only the single item currently being handled.
- **Before and after**: `const data = await getUser(id)` → `const customer = await findCustomerById(id)`
- **blocking**: None (record as reference information).
- **Evidence**: The scope of responsibility can be stated in one sentence from the name.

## 4. Do not write comments

- **Applicability**: When writing comments or deciding whether to keep them.
- **Exemptions**:
  - Comments required for license notices or code-generation integration. Show the grounds for retaining them in the artifact.
  - Directive comments read as input by compilers, build tools, or analysis tools. This does not exempt the validity of suppressing checks itself. Put the reason for suppression on the same line.
- **Principle**: Do not write comments in code. Express design through module divisions, names, types, and data structures. Before explaining something in a comment, consider whether structure can express it; if so, correct the structure. Designs that need comments usually have structural errors. Only two kinds of comments are permitted:
  - Code working around a specific behavior or defect in an external library, tool, or platform. State what is being avoided. For defects, include removal conditions such as a fixed version or upstream issue.
  - Deliberately unnatural implementations required by particular circumstances. State those circumstances and when the implementation can be removed.
- **Violations and signs**:
  - Comments that restate code, such as `// Get the user` above `getUser()`.
  - Comments above functions describing arguments, return values, or behavior when arguments are omitted.
  - Explaining design reasons already visible from structure, such as append-only storage or key ownership.
  - Compensating with comments for design reasons that structure fails to express, without correcting the structure.
  - History comments such as "Changed to..." or "Fixed..."
  - Commented-out code or TODOs remain.
- **Before and after**: A comment saying "Newest first" above `listVersions()` → Name the function `listVersionsNewestFirst()`.
- **blocking**: A comment is added that fits neither of the two permitted kinds nor an exemption.
- **Evidence**: For each retained comment, identify the permitted kind or exemption.

## 5. Investigate affected uses and errors with the same cause

- **Applicability**: When changing functions, types, contracts, names, paths, commands, settings, or procedures. When fixing defects or about to add guards (value checks).
- **Exemptions**: None.
- **Principle**: Based on confirmed causes, investigate uses, locations that depend on previous assumptions, stored data, and errors with the same cause. Uses include documents referencing changed names, paths, or commands and documents explaining changed behavior, settings, or procedures. For defects appearing only after restart, investigate persisted state too. Follow principle 3 of `verification.md` for observing causes. Fix the parts of confirmed causes that can be controlled or changed, and identify fixes inseparable from establishing the changed contract or invariants. Determine the fix scope from the request's scope and necessary consistency, rather than placement. Follow principles 6 and 7 of `working.md` for permission to change; confirm a scope change if inseparable fixes exceed the request. Do not include independent defects in the same diff merely because of their location or relation to security; report them with their impact. Disappearance of symptoms alone is not evidence that a cause has been removed.
- **Violations and signs**:
  - Fixing only the noticed location without investigating uses of the same function or errors with the same cause.
  - Changing a contract without investigating effects on consumers or stored data.
  - Including independent defects in the same diff merely because they are in the same module or relate to security.
  - Calling a contract change complete while leaving affected locations known to require changes unaddressed.
  - Hiding symptoms with a guard without verifying the cause, and treating the cause as removed too.
  - Not stating remaining limitations of workarounds or conditions for their removal.
  - Changing files or commands while leaving references in repository instructions, READMEs, or skills outdated.
- **Before and after**: "Changed the return type of `parseDate` to `Date | null`" → "Investigated effects on callers and stored data, and made changes essential to the new contract within the request's scope. Reported independent defects with their impact."
- **blocking**: A contract is changed without investigating effects on consumers or stored data, or work is called complete while affected locations confirmed as necessary for the contract or invariants remain unaddressed.
- **Evidence**: References to observations confirming the cause, investigation results for affected uses and errors with the same cause, each response, and reasons for the fix scope. For workarounds, show effects that can be isolated, remaining limitations, and removal conditions.

## 6. Pass only the authority and information needed

- **Applicability**: When passing information or operation authority to external services, clients, other processes, and so on.
- **Exemptions**: None.
- **Principle**: Identify the information and authority required for the recipient's processing, and limit what is passed to that scope. Internally accessible records or available powerful credentials are not reasons to pass them as-is. Some information cannot be retrieved or restricted after disclosure, so determine its necessity before passing it.
- **Violations and signs**:
  - Sending entire internal records to external processing that needs only some fields.
  - Passing update and deletion permissions to read-only processing.
  - Passing credentials with access to everything when access can be limited to the target.
- **Before and after**: `translate(customer)` → `translate({ displayName: customer.displayName })`
- **blocking**: Confidential information or authority passed by the diff exceeds the information and authority used by the recipient's processing as stated in the evidence.
- **Evidence**: For each piece of information or authority passed, show its relationship to the processing that requires it. If a narrower scope cannot satisfy the processing, explain why.

## 7. List required external state and execution order

- **Applicability**: When changing environment variables, secrets, permissions such as service or database roles, infrastructure resources, migrations, or deployment order.
- **Exemptions**: None.
- **Principle**: List external state required for the change to work: environment variables and where their values come from, secrets, permissions, and external service settings. Specify the order of infrastructure application, migrations, and deployment. Repository tests cannot verify external state; the wrong order after merging can fail deployment or leave old settings in use. When renaming environment variables or secrets, specify how to handle old names remaining in developer environments, CI, or hosted environments.
- **Violations and signs**:
  - Adding code that reads an environment variable without stating where its value is set.
  - Omitting order when deployment would fail before infrastructure changes are applied.
  - Renaming an environment variable while tests silently skip in environments still configured with the old name.
- **Before and after**: "Added a webhook service" → "Added a webhook service. After merging, apply infrastructure changes before deploying the service. Store the new `WEBHOOK_SECRET` in the secret store."
- **blocking**: A change requires external state but omits its requirements or execution order.
- **Evidence**: Required external state, who prepares each item and when, and the order of infrastructure application, migrations, and deployment.

## 8. Share definitions of rules that must change together

- **Applicability**: When adding functions, conversions, checks, or rules, or considering sharing similar processing.
- **Exemptions**: None.
- **Principle**: Processing that must always reflect changes to the same rule simultaneously must share the rule's definition; keep processing independent when its reasons for change or contracts differ, even if its text is identical. Search for existing definitions before adding processing that does the same. Decide whether the processing must change together, rather than whether its text looks similar. Reuse the existing definition when the rule or behaviour must change together. Similar code is preferable to premature abstraction when the reasons for change are independent.
- **Violations and signs**:
  - Added code re-implements an existing function, conversion, check, or rule that must change together with it.
  - Multiple modules independently define format or ordering rules that must change together.
  - Multiple processing paths must apply the same rule, but there is no guarantee that changes are reflected simultaneously.
  - Combining separate rules in one place merely because they happen to have the same shape (they will need separating again when only one must change).
- **Before and after**: Re-implement the customer-name conversion used by receipts in a new invoice path that must follow the same display rule → Reuse the existing conversion; keep search normalization independent because its contract changes for different reasons.
- **blocking**: Added code re-implements a rule or behaviour already defined elsewhere that must change together with it.
- **Evidence**: Locations searched and existing definitions found. The shared rule or behaviour and why its uses must change together, or the different reasons for change or contracts that require independent processing.
