Stores stage definitions passed to brief assembly.
Plan semantics belong in internal/plan, brief assembly in internal/brief, and progression in internal/engine.
For placement, see `docs/architecture/modules.md#kit`; for assembly, see `docs/architecture/modules.md#brief`.

A stage definition is Markdown named `<stage_id>.md`, with an initial `# <name>` and the following headings in order: `## Inputs`, `## Decisions`, `## Deferred to later stages`, `## Irreversible items`, `## Artifacts`, and `## Completion conditions`. The filename is the ID; each section is machine-readable through the next second-level heading. The corresponding stage in `docs/stages.md` is the source of truth; update the corresponding definition when changing it. Code-formatted references starting with `docs/` or `principles/` are relative to the repository root during development and to the kit copy root after installation. Inputs follow `docs/parts/brief.md`, and common completion conditions follow `docs/architecture/guarantees.md#%E5%AE%8C%E4%BA%86%E6%9D%A1%E4%BB%B6`. The plan template and plan determine adopted stages, order, checks, runners, gates, limits, and transitions.

- `purpose.md`: 0 Purpose.
- `scope.md`: 1 Scope.
- `structure.md`: 2 Structure.
- `contract.md`: 3 Contract.
- `steps.md`: 4 Steps.
- `implement.md`: 5 Implementation.
- `research.md`: Research.
- `prototype.md`: Prototype.
- `initialize.md`: Initialization.
