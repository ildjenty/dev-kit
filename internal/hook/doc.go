// hook deterministically processes the host's five events and writes observations and results as generated files.
// It performs no model-based judgments and receives the resolved root, work ID, and normative readers from cmd/devkit.
// It depends on ledger, config, host/event, form, and ledger/fsstore.
// Source of truth: docs/architecture/modules.md#hook
package hook
