// external is the sole execution path for external writes managed by kit, handling approval, records before and after execution, execution, and recovery from unknown outcomes.
// It has no implementations for individual operation types and invokes registered command templates in the checkout.
// It depends on ledger and receives command registrations as values loaded by config.
// Source of truth: docs/architecture/modules.md#external
package external
