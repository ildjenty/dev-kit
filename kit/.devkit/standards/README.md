# Repository standards

Use these principles and review criteria to guide design, implementation, verification, and judgment. After installation, the repository owns and maintains this folder; kit updates do not replace its existing standards. Workflow checks select applicable principles and review criteria from `review/`.

Keep mechanically decidable rules in checks rather than principles. Put workflow execution rules in workflow definitions. Enforce rules for target code and documents through types, lint, dependency checks, tests, or scripts. Keep only short, broadly applicable rules that cannot be enforced mechanically in repository instructions.

## Documents

- [README.md](README.md): Standard ownership, principle format, conflict resolution, and updates.
- [authority.md](authority.md): Decisions reserved for the owner and delegated to agents.
- [git.md](git.md): Request branches, worktrees, commit prefixes, and owner-controlled merging.
- [engineering.md](engineering.md): Failures, reruns, concurrency, naming, comments, change impact, permissions, external prerequisites, and definitions of rules that must change together.
- [designing.md](designing.md): Decision order, abstraction, integration, migrations, necessary mechanisms, and conventions.
- [structure/purpose.md](structure/purpose.md): The five purposes of structure and their priority.
- [structure/smells.md](structure/smells.md): Observations that structural purposes are failing.
- [structure/deciding.md](structure/deciding.md): Placement, ownership, divisions, dependency direction, and when to fix structural defects.
- [structure/principles.md](structure/principles.md): Module responsibilities, boundaries, guarantees, public contracts, and understanding cost.
- [data.md](data.md): Sources of truth, recovery, valid states, ownership, concurrency, and meaning preservation.
- [testing.md](testing.md): Test selection, contracts, failure detection, reproducibility, and behavior preservation.
- [verification.md](verification.md): Primary evidence, inference, causal investigation, and measurement claims.
- [security.md](security.md): Untrusted input, secrets, authentication, authorization, permissions, dependencies, and records.
- [working.md](working.md): Scope, sequencing, verification, routine commands, approval, and delegated execution.
- [context.md](context.md): Durable work, memory, untrusted instructions, reading scope, and safe handoffs.
- [harness.md](harness.md): Independent judgment, returning to earlier stages, enforcement, and mechanism costs.
- [ui.md](ui.md): User purpose, information, interactions, recovery, accessibility, evidence, and approval.
- [writing.md](writing.md): Decision-ready reporting, reader assumptions, concrete wording, consistency, and disclosure.
- [review/README.md](review/README.md): Criteria selection and common rules for every judgment.
- [review/purpose.md](review/purpose.md): Request coverage, unresolved requirements, and judgeable completion.
- [review/structure.md](review/structure.md): Placement, dependencies, boundaries, simplicity, and architecture smells.
- [review/wording.md](review/wording.md): Reader assumptions and specific, understandable wording.
- [review/contract.md](review/contract.md): Accepted states, failures, public responses, reruns, and concurrency.
- [review/security.md](review/security.md): Security properties of changed code and passed authority or information.
- [review/report.md](review/report.md): PR format, decision information, verification claims, and agreement with evidence.
- [review/test.md](review/test.md): Executable tests of assigned completion conditions and public contracts.
- [review/harness.md](review/harness.md): Workflow mechanisms, skills, judgment, retrospectives, continuity, and burden.

## Concern ownership

`concerns.json` is the single map of quality responsibilities, principle ownership and plan/code criteria. Each numbered principle is owned exactly once by a quality, `every_quality`, or a named writer or non-quality judge. Every review document except `review/README.md` is named in the map. Run `devkit concerns check` after changing standards. Fixed snapshots are checked before plan, judge and refine prepare.

- [review/data.md](review/data.md): How facts, valid states, ownership and lifetime preserve meaning and invariants.
- [review/ui.md](review/ui.md): How users understand and complete their task through visible states and interactions.
- [review/cost.md](review/cost.md): Which performance, observability, rollout and operational measures justify their cost.
- [review/delivery.md](review/delivery.md): How ordered release units and verification cover the request end to end.
- [review/scope.md](review/scope.md): Which changes fulfill the agreed request and preserve affected uses within authority.
- [review/readability.md](review/readability.md): Which names, comments, conventions and shared rule definitions make the changed code understandable and keep rules that must change together consistent.
- [review/document.md](review/document.md): How delivered documents support their readers with context, actionable instructions, consistent sources and appropriate disclosure.

## Principle format

Each principle uses the following fields:

- **Name**: A short name usable in conversation, such as "Do not abstract until there is a second concrete example."
- **Applicability**: One sentence identifying when the principle applies.
- **Exemptions**: Conditions under which following a requirement would worsen the situation, the exact requirement waived, and what the artifact must show to establish the condition. Use nested bullets for multiple conditions. An exemption waives only its stated requirement; other requirements and blocking conditions remain effective. Do not list ordinary actions already permitted by the principle. Write "None" when no exemption is known; counterexamples may still challenge it.
- **Principle**: What to do and why.
- **Violations and signs**: Observable conditions for examining a diff or proposal.
- **Before and after**: One pair illustrating improvement.
- **blocking**: Conditions under which a check treats the violation as blocking. When a condition asks whether evidence states something, check its presence at plan time; at code time, also verify that the stated evidence matches the code.
- **Evidence**: Grounds showing that the principle was satisfied.

In responses, state which principle changed which decision. Naming a principle without changing a decision does not count as applying it. When using an exemption, identify it, the requirement waived, and the reason.

## When principles conflict

- **Design reasons and code comments**: Under principle 1 of `structure/principles.md` and principle 4 of `engineering.md`, keep design reasons that structure cannot express in plan evidence or repository documents. Reference them from where needed rather than adding code comments.
- **Abstraction and external types**: Apply principle 5 of `structure/principles.md` as the exemption to principle 2 of `designing.md`. Convert external types at boundaries before a second concrete example when necessary to isolate public contracts, stored data, and domain types from external details. Do not force conversion where principle 5 exempts general-purpose value types or other qualifying uses. Show that hidden knowledge and the reach of changes justify conversion and maintenance costs. Do not extend this into substitution interfaces.
- **Sources of truth and information hiding**: Prioritize principle 1 of `data.md` over principle 6 of `structure/principles.md`. Preserve facts from past points in time. Performance-driven duplication requires a source of truth, synchronization ownership, update methods, permitted inconsistency, and a response when that scope is exceeded. Information hiding determines what is public; keep storage methods inside their responsible module.
- **Universal guarantees and consumer independence**: Prioritize principle 3 over principle 4 of `structure/principles.md`. Cross-cutting processing must not know consumer names; use generic middleware. Put consumer-specific branches in the consumers rather than the shared passage point.
- **Removal and requested scope**: Prioritize principle 7 of `working.md` over principle 5 of `designing.md`. Removal is a change. Consider items made unnecessary by the request or whose removal is needed to satisfy it, without mixing in unrelated cleanup. Report out-of-scope removal as a proposal.
- **Destination defects and requested scope**: Treat defects at the destination level and along its ancestor path in `structure/deciding.md` as fixes inseparable from the request under principle 7 of `working.md`; fix them in the same work. Report defects outside that path as proposals. Adding to a defective destination creates another instance of the defect, so correcting that destination is inseparable from the requested change. Show the repair scope and the additional defect that leaving it unrepaired would create. Verify these behavior-preserving structural changes separately from requested behavior changes under principle 3 of `working.md`. Ask the owner under `authority.md` when a repair changes module boundaries or dependency direction.
- **Conventions and migration**: Prioritize principle 4 of `designing.md` over principle 7 within the agreed migration scope for changed conventions. Otherwise follow existing conventions that satisfy current requirements and guarantees under principle 7. Limit migration to changes inseparable from the request; leave no compatibility layer where a safe all-at-once switch is possible. If migration must be staged, define temporary compatibility scope and removal conditions. Treat independent efforts to align the whole repository as separate issues.

## Updating principles

Add or change only a few principles at once. Generalize lessons beyond individual events. Extend an existing principle when it already covers the issue rather than creating an overlapping one. During retrospectives, use observed problems, missed issues, gaps, and burden to check which checks and templates select each principle, whether principles are unused, and whether their cost exceeds their benefit; consider consolidation or reduction. Review the entire standards only when a need becomes evident. Treat principle changes as workflow changes and, as a rule, obtain owner review before first applying them.
