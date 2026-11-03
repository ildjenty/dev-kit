// engine advances the state machine, computes instructions, runs judge and verification commands, accepts and records results, and writes briefs.
// Git operations belong to workspace, external writes to external, and storage to ledger/fsstore; engine does not launch subagents.
// It depends on plan, brief, check, router, ledger, form, workspace, external, and ledger/fsstore.
// Source of truth: docs/architecture/modules.md#engine
package engine
