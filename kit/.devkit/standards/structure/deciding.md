# Structural decisions

Use the purposes in `structure/purpose.md` and the architecture smells in `structure/smells.md`.

## Determine the item's nature

Answer these questions with evidence such as file paths, request text, or calling relationships. Write "unknown" for answers without evidence.

- **Kind**: Complete "X is a kind of ..." with its role in composition: an entry point receiving calls, orchestration determining order, a component doing one job, a definition containing data and pure decisions, a connection to an external system, or a contract both sides honor. Choose a noun that could have a second item of the same kind under the same parent.
- **Reasons for change**: List everyone whose changes would require modifying the item, and what they would change.
- **Volatility**: Determine it from the source of change. Product flows and requirements change more readily than external services and libraries, which change more readily than domain knowledge, which changes more readily than general types and mechanisms.
- **Required knowledge**: What imports are needed to write it: only external libraries, repository types too, or domain knowledge too?
- **Callers and runtime location**: Who calls it: people, external services, other services, or scheduled execution? In which deployment unit does it run?
- **Failure and authority scope**: What data can it read or write, what permissions does it hold, and what stops if it fails?
- **Lifetime**: Will it remain, be discarded after experimentation, or eventually move elsewhere?
- **Consumers and growth**: How many consumers use it now, and what growth creates more of these items?

## Division and dependency direction

- Structure forms a tree. At every level, divide children by one criterion. Siblings are different things classified by that same criterion. Determine placement by answering, from the root, "What divides this level, and which category contains this item?"
- There are two trees: code contains changes; running components contain failures and authority. They overlap at deployment units. At those intersections, first define boundaries containing failures and authority, then divide their interiors by reasons for change.
- At each level, choose a criterion whose resulting groups change independently. Higher levels use boundaries whose violation is harder to recover from.
- A module has one reason for change and hides it. Put things that change together in the same module, and independently changing things in separate modules. Do not create a module that hides nothing.
- Every item in a module must be describable as a kind of the thing named by the module. List reasons for change for each item, rather than only for the module; separate items with different owners and return them to those owners.
- Do not move an existing block of code intact. Separate its contents by owner before deciding each location.
- Dependencies point only from volatile, short-lived, specific components toward stable, long-lived, general components.
- Keep one contract where only its two parties can see it, at their lowest common parent. If there is only one consumer, place it with the called component.
- Share components only after they become more stable than their consumers. Do not combine things that change for different reasons for each consumer.
- Build temporary items destined to move using the destination's divisions, so moving requires no rewriting. Isolate experiments that may be discarded from production components so they can be deleted as a whole.
- Requested placement, divisions, or mechanisms remain subjects to verify. If they undermine the purposes, propose revising the request rather than following the specified structure.
- Documentation and existing structural rules reflect past decisions. When using a rule to reject an option, state which purpose it preserves. If no purpose can be stated, or the rule undermines it, consider correcting the rule.

## When to fix discovered defects

- **In the current work**: Fix defects at the level receiving the new item and along its ancestor path. Adding without fixing creates another instance of the defect, such as another item in an undivided collection or another mixed responsibility. Deferral increases what must later move.
- **In separate work**: Report defects outside the new item's path with what should change and how.
- Do not defer defects in the destination using the separate-work category.
