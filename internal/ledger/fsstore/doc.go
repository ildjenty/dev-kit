// Package fsstore handles record and artifact storage, publication, corruption detection, and work locking.
// It handles only the supplied record root; ledger owns record semantics and cmd/devkit resolves roots.
// It does not depend on other modules.
// Canonical design: docs/architecture/modules.md#ledgerfsstore
package fsstore
