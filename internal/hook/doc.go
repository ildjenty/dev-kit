// hook deterministically processes the host's five events and writes observations and results as generated files.
// It performs no model-based judgments and receives function values for reading the root, work ID, and norm configuration from cmd/devkit.
// It depends on ledger, config, host/event, form, and ledger/fsstore.
// Source of truth: docs/architecture/modules.md#hook
package hook
