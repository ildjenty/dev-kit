// Package brief assembles briefs and requests from supplied stage definitions, plans,
// previous outputs, and repository facts. check selects judgment request content;
// engine writes request files. brief performs no I/O and uses request templates
// supplied by the caller. It depends on plan for stage types and form for templates.
// Reference: docs/architecture/modules.md#brief
package brief
