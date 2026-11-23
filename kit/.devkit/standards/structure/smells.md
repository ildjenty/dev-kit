# Architecture smells

Architecture smells are observations that the five purposes in `structure/purpose.md` are failing, extending code smells to design and structure. Smells are observations rather than rules. A failing purpose still warrants correction when none of these smells appears.

- **Failures and authority escape**: Components with different accessible data or permissions run in the same service with the same authority. Code branches protect boundaries that need deployment separation. Contracts or types are visible to parties that do not need them.
- **Replacement or deletion is difficult**: Replacing a module requires rewriting its surroundings. Deleting it leaves configuration, registrations, or types behind. Dependencies point from stable to volatile components, or form cycles. Temporary components have a different structure from their eventual destination.
- **Changes spread**: Adding another item of the same kind requires changing foundation code or existing siblings. One reason for change spans multiple modules. Unrelated changes conflict in the same location. Components with different reasons for change share a type or function merely because they have the same shape, or are forced into that shape.
- **Placement is ambiguous**: An item fits two locations. Siblings use different classification criteria. A flat collection has no criterion for additions. A module mixes different kinds of things.
- **Purpose is unclear**: Names reveal neither kind nor purpose. Some contents do not belong to the kind named by their module. Files mirror another module's divisions one for one, such as one file per module in another package. Layers or folders hide nothing. Placement is explained by convenience rather than the item's nature.
- **A mechanism serves none of the purposes**: A mechanism, path, or setting has no current subject to protect, consumer, or use. Custom code recreates a standard capability of the framework or library already in use.
- **Workarounds bypass structure**: Re-exports, relays, type-only files, or rule exceptions evade placement or dependency rules.

Convenience arguments indicate failing purposes: "because it cannot be imported," "to avoid adding permissions," "because it is already there," or "because the documentation says so." Workarounds for those arguments have the same problem. If a workaround is needed, either the placement or the rule is wrong. Correct the wrong one.
