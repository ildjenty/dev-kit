# Decision authority

Obtain the owner's approval before first applying changes to decision authority, as changes to the workflow itself.

## Stop and ask the owner

Do not proceed with work that depends on any of these decisions:

- Difficult-to-change or wide-reaching choices, including technologies, architecture, data models, public interfaces, module boundaries, and dependency direction. Follow principle 1 of `designing.md`.
- Decisions not settled by requirements, external contracts, or conventions that affect completion conditions or externally visible behavior.
- Departures from conventions.
- Decisions affecting cost, operations, security, personal information, or billing. Cost changes include adding paid calls or operation, relaxing limits on calls, fixes, retries, delays before retrying after failure, scheduled execution frequency, instance counts and resources, or database service tiers, and removing cost controls. Show the increase when asking. Do not ask again within a previously approved quantity and scope.
- Actual writes to external systems. Posting, sending, and verification that incurs charges also require explicit permission.
- Resolving discrepancies with requirements documents or changing those documents.
- Changes to conventions, operating rules, skills, or CI rules.
- Deletion or overwriting that git cannot restore, deployment, or merging.
- Choices determined only by the owner's preferences.

If completion conditions prove excessive, premature, or inconsistent with reality, stop work intended to satisfy them and report the issue. The owner decides whether to revise them.

## Delegated decisions

Explicit delegations take precedence over overlapping categories above. For new delegations, retain the decision category and grounds from the owner's instructions or recorded answers.

### Proceed provisionally for owner review

- Easily reversible design decisions with multiple options that structure or conventions do not settle.
- Matters the AI reports uncertainty about that do not fall under "Stop and ask the owner."
- Matters whose reversibility is uncertain; report why. If database schemas, public APIs, or external writes are involved, stop and ask when uncertain.

Record provisional choices and reasons in the plan and list them in the PR body. Only easily reversible decisions qualify. Irreversible changes, changes to checks, and new packages belong in the PR body's mechanically assembled checkpoints. When reporting in conversation, explicitly identify limits, defaults, failure behavior, or other choices absent from instructions and documents as decisions you made.

### Changes that meet the same purpose at least as well

Evaluate conventions by their purpose as well as their wording. A simpler method that meets the purpose at least as well may proceed provisionally. Include the convention's documentation in the same change, and record the purpose and why the method improves on the previous one. Stop and ask if it cannot meet the purpose or separately involves an approval category above. Do not silently change documentation to match code or retain defects merely to match documentation.

### Changes the owner need not inspect as diffs

Within agreed and delegated scope, the following may proceed after AI review and CI:

- Implementation details determined by the agreed structure.
- Identifying convention violations and making fixes whose method is clear.
- Adding tests, formatting, and mechanical refactoring.

Changes reversible through git, commits on the work branch, pushes, and PR creation may proceed without asking if they contain no approval matters. The same applies to visual prototypes, fixing defects you introduced, and cleaning up temporary files. The owner decides merging; follow `git.md` for branches and integration.

Summarize changes even when the owner need not inspect their diffs, so they can understand the work without opening it.

### Owner review of foundation and feature changes

Distinguish foundation from feature changes by the reach of errors, rather than by package location. Changes whose errors spread to all features are foundation changes even inside a feature package. Before merging foundation changes, the owner reads and understands a mechanically assembled list of structural changes in the PR body: added and removed folders with their siblings and immediate file counts, added and removed packages, changes in dependencies between packages, changes to dependency rules, and moves across packages or folders. Errors in foundations affect all features, and investigating their causes requires understanding the structure. The owner retains module-boundary and dependency-direction decisions until judge accuracy is measured for each category of delegated decision and meets the required conditions. For feature changes, the owner decides merging from AI review and the PR body.

## Requesting a decision

First show what previous owner instructions already settle, and ask only about what remains unresolved. Provide short options, a recommendation, and its reason. A delegated agent that encounters an approval matter returns it to the delegating agent without deciding or proceeding provisionally. The delegating agent asks the owner.

## Judgment of authority

Classifications must match these authority rules and recorded delegations. Finalized owner decisions require matching answers or approval-point approval. Proposed decisions at the current approval point are provisional, with options, a recommendation and reasons. Later outputs conform to committed approved decisions. A finalized owner decision without its matching approval, an owner matter neither settled nor proposed at the current approval point, or later work departing from committed approved decisions blocks.
