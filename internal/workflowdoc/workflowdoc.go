package workflowdoc

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var Order = []string{"optimize", "refine", "requirements", "plan", "implement", "judge", "fix", "pr", "sync", "retro"}

type node struct {
	ID        string            `yaml:"id"`
	DependsOn []string          `yaml:"depends_on"`
	When      string            `yaml:"when"`
	Bash      string            `yaml:"bash"`
	Command   string            `yaml:"command"`
	Prompt    string            `yaml:"prompt"`
	With      map[string]string `yaml:"with"`
	Loop      *struct {
		MaxIterations int    `yaml:"max_iterations"`
		Nodes         []node `yaml:"nodes"`
	} `yaml:"loop_group"`
}

type workflow struct {
	Description string `yaml:"description"`
	Nodes       []node `yaml:"nodes"`
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_]`)
var assignment = regexp.MustCompile(`^\S+=\S+\s+`)

func id(s string) string { return "n_" + unsafe.ReplaceAllString(s, "_") }

func text(s string) string {
	return strings.NewReplacer(`"`, "'", "<", "‹", ">", "›").Replace(s)
}

func ai(n node) bool { return n.Command != "" || n.Prompt != "" }

func label(n node) string {
	var sub string
	if ai(n) {
		sub = "AI: " + n.Command
		if n.Command == "" {
			sub = "AI: prompt"
		}
		if q := n.With["quality"]; q != "" {
			sub += " (" + q + ")"
		}
	} else {
		lines := strings.Split(strings.TrimSpace(n.Bash), "\n")
		cmd := strings.TrimSpace(lines[len(lines)-1])
		for _, l := range lines {
			if strings.Contains(l, "devkit") {
				cmd = strings.TrimSpace(l)
				break
			}
		}
		sub = assignment.ReplaceAllString(cmd, "")
		if r := []rune(sub); len(r) > 50 {
			sub = string(r[:50])
		}
	}
	return "<b>" + text(n.ID) + "</b><br/><small>" + text(sub) + "</small>"
}

func emit(nodes []node, out *strings.Builder, indent, prefix string) {
	for _, n := range nodes {
		nid := id(prefix + n.ID)
		if n.Loop != nil {
			fmt.Fprintf(out, "%ssubgraph %s [\"%s (loop, max %d)\"]\n%s  direction TB\n", indent, nid, text(n.ID), n.Loop.MaxIterations, indent)
			emit(n.Loop.Nodes, out, indent+"  ", prefix+n.ID+"_")
			fmt.Fprintf(out, "%send\n%sclass %s loop\n", indent, indent, nid)
			continue
		}
		class := "machine"
		if ai(n) {
			class = "ai"
		}
		fmt.Fprintf(out, "%s%s[\"%s\"]:::%s\n", indent, nid, label(n), class)
	}
	known := map[string]bool{}
	for _, n := range nodes {
		known[n.ID] = true
	}
	for _, n := range nodes {
		for _, d := range n.DependsOn {
			if !known[d] {
				continue
			}
			arrow := " --> "
			if n.When != "" {
				arrow = " -.-> "
			}
			fmt.Fprintf(out, "%s%s%s%s\n", indent, id(prefix+d), arrow, id(prefix+n.ID))
		}
	}
}

// Render builds docs/workflows.md from the workflow definitions under dir in files.
func Render(files fs.FS, dir string) (string, error) {
	var out strings.Builder
	out.WriteString("# Workflow diagrams\n\n")
	out.WriteString("Generated from `kit/.archon/workflows/dev/*/*.yaml`. After changing a workflow, regenerate with `DEVKIT_UPDATE_DOCS=1 mise exec -- go test ./internal/workflowdoc`; the test fails while this file is out of date.\n\n")
	out.WriteString("Gray nodes run on the machine (bash and devkit), blue nodes are AI nodes, and yellow frames are loops. A dotted arrow leads to a node that runs only under a condition (`when`). See the [design document](design.md) for each workflow's role.\n")
	for _, name := range Order {
		data, err := fs.ReadFile(files, path.Join(dir, name, name+".yaml"))
		if err != nil {
			return "", err
		}
		var wf workflow
		if err := yaml.Unmarshal(data, &wf); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		fmt.Fprintf(&out, "\n## %s\n\n%s\n\n```mermaid\nflowchart TD\n", name, strings.Join(strings.Fields(wf.Description), " "))
		out.WriteString("  classDef machine fill:#f5f5f5,stroke:#666,color:#333\n  classDef ai fill:#dae8fc,stroke:#6c8ebf,color:#000\n  classDef loop fill:#fff8e1,stroke:#d6b656\n")
		emit(wf.Nodes, &out, "  ", "")
		out.WriteString("```\n")
	}
	return out.String(), nil
}
