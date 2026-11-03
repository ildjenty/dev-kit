// view generates static HTML for people to view one work item from ledger and parsed structure values.
// It provides no server or state changes from HTML; cmd/devkit retrieves structure history through workspace and passes it to view.
// It depends on ledger, form for rendering questions, and plan for stage names.
// Generate accepts a loaded ledger, its ordered records, resolved decisions, and the current comparison context.
// Structure and StructureRevision carry parsed structure data; StructureMermaid renders dependency text.
// Source of truth: docs/architecture/modules.md#view
package view
