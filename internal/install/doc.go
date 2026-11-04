// install handles installation layout and configuration additions and registration through init, their removal through uninstall, and verification through doctor.
// It handles existing configuration and skills idempotently without overwriting them; kit supplies, copies, and verifies matches of fixed assets.
// It depends on config, kit, ledger, and workspace.
// Source of truth: docs/architecture/modules.md#install
package install
