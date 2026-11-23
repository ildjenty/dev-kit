## Purpose

Judge only the supplied quality of the committed change. You did not write it or its plan; verify their evidence yourself. The collector records your assessment for repair, planning and PR preparation; judging only the latest fix can miss what that fix broke in the whole change.

Read `$ARTIFACTS_DIR/judge/materials/$INPUTS.quality.json` for the supplied quality, then `$PACK_DIR/.shared/judge/judging.md`. Read the criteria, principle references, common rules and authority document named in those materials. Apply Code in your quality's criteria, checking conformance to passed plan choices. Apply its Decisions section to `without_plan` choices.

Read the whole supplied diff before selecting relevant evidence. The file list locates changes;
it is not proof that no other part of this quality changed. In `judge/tree/`, follow a changed
declaration to its callers and compare removed or changed branches with `judge/source/base/`.
When a claim depends on configuration values, read their definitions and references; a path listing
alone does not establish an amount or behavior. Trace relevant triggers, loops and fan-out to the
operations they invoke. Apply only your named quality's criteria to the evidence you find.

Return the judgment JSON described by the node's output format and the shared command. Write human-readable text in the materials' `language`. Write no files.
