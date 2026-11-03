// Package plan handles plan semantics without I/O: validation, deviations, transitions, limits, and replanning.
// It also determines executable plans and matches external operations covered by plan approval. Storage belongs in ledger; progression belongs in engine.
// It does not depend on other modules.
// Source of truth: docs/architecture/modules.md#plan
package plan
