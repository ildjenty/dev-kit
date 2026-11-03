// external is the sole execution path for external writes managed by kit, handling approval, records before and after execution, execution, and recovery from unknown outcomes.
// It has no implementations for individual operation types and invokes registered command templates in the checkout.
// It depends on ledger and receives command registrations as values loaded by config.
// Call Describe to construct approval targets, Execute to perform an approved write,
// and Recover to query an interrupted or unknown outcome without repeating the write.
// Execute and Recover require the caller to hold the work lock.
// Registered commands receive JSON on stdin and the operation ID in
// DEVKIT_OPERATION_ID. They return a JSON object on stdout with status
// (succeeded, failed, or unknown), an optional result string, and an optional
// external_id string. Query commands use the same protocol and must only query.
// Source of truth: docs/architecture/modules.md#external
package external
