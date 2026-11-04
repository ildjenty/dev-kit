// Package check resolves criteria selections, prepares judgment requests, and accepts and aggregates results without I/O.
// It assesses exemptions and required checks. Command execution belongs in engine, storage in ledger, and routing in router.
// Accepted finding IDs include their aspect namespace; correction requests use aspect-local IDs.
// The aspect only names a judgment; its criteria are the documents selected by the check.
// It depends on form for judgment result schemas.
// Source of truth: docs/architecture/modules.md#check
package check
