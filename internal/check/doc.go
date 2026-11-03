// Package check parses aspects, prepares judgment requests, and accepts and aggregates results without I/O.
// It assesses exemptions, required checks, and detection tests. Command execution belongs in engine, storage in ledger, and routing in router.
// It depends on form for aspect formats and judgment result schemas.
// Source of truth: docs/architecture/modules.md#check
package check
