package view

import (
	"fmt"
	"strings"
)

// Structure is the parsed module, responsibility, placement, and dependency data.
// Version is the content hash supplied by the caller, not a hash of this value.
type Structure struct {
	Version      string       `json:"version"`
	Modules      []Module     `json:"modules"`
	Dependencies []Dependency `json:"dependencies"`
}

type Module struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Responsibility string `json:"responsibility"`
	Path           string `json:"path"`
}

// Dependency points from a dependent module to the module it depends on.
type Dependency struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// StructureRevision is parsed git history supplied by cmd/devkit, newest first.
// A nil Structure represents a deletion.
type StructureRevision struct {
	Commit    string
	Structure *Structure
}

// StructureMermaid returns dependency diagram text without executing Mermaid.
func StructureMermaid(structure Structure) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	ids := make(map[string]string, len(structure.Modules))
	for i, module := range structure.Modules {
		id := fmt.Sprintf("m%d", i)
		ids[module.ID] = id
		label := module.Name
		if label == "" {
			label = module.ID
		}
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", id, mermaidLabel(label))
	}
	for _, dependency := range structure.Dependencies {
		fmt.Fprintf(&b, "  %s -->|depends on| %s\n", ids[dependency.From], ids[dependency.To])
	}
	return b.String()
}

func mermaidLabel(value string) string {
	var b strings.Builder
	for _, r := range value {
		// Numeric Mermaid entities keep labels from becoming diagram syntax.
		fmt.Fprintf(&b, "#%d;", r)
	}
	return b.String()
}
