// Package router selects actions from supplied classifications, aggregated judgments, progress, and limits, and handles completion conditions and human confirmation.
// It classifies and decides without I/O; loop progression following decisions belongs in engine.
// It depends on plan for limits and transitions and check for judgment types.
// Source of truth: docs/architecture/modules.md#router
package router
