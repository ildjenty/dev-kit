// main defines subcommands and wires their implementations together.
// It resolves the root, branch, session, work, and configuration source, and passes configuration and kit values.
// It depends on all implementation packages; plan, judgment, and record semantics belong to their respective modules.
// Source of truth: docs/architecture/modules.md#cmddevkit
package main
