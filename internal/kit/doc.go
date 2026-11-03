// kit supplies assets from a repository copy, copies embedded assets for init, and verifies them for doctor.
// Embedding belongs to the root package; state machine progression and domain decisions without I/O do not belong here.
// This package reads embedded assets from the root package and does not depend on other processing modules.
// Source of truth: docs/architecture/modules.md#kit
package kit
