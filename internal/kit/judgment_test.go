package kit_test

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"text/template"

	"github.com/ildjenty/dev-kit/internal/kit"
)

func TestJudgmentCriteriaAndDetectionInputs(t *testing.T) {
	data, err := fs.ReadFile(kit.FS(), "templates/brief/judgment.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("judgment").Option("missingkey=error").Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, detection := range []bool{false, true} {
		input := map[string]any{
			"InstructionID": "instruction-1", "InstructionIDJSON": `"instruction-1"`,
			"Content": "Anonymous target", "OutputSchema": "Output contract", "Detection": detection,
			"Criteria": []map[string]string{{"Path": "principles/review/test.md", "Content": "Selected frozen criteria"}},
			"Repository": map[string]any{
				"Requirements": map[string]any{"Path": "requirements.md", "Exists": true, "Content": "Repository product body"},
				"Structure":    map[string]any{"Path": "structure.json", "Exists": false},
				"Rules":        map[string]string{"Path": "rules.md", "Content": "Repository rule body"},
				"LayoutGuides": []map[string]string{},
			},
			"Norms":     []map[string]string{{"Path": "examples/spec.json", "Content": "Hidden specification"}},
			"NormPaths": []map[string]string{{"Path": "history", "SnapshotPath": "Hidden history path"}},
		}
		var output bytes.Buffer
		if err := tmpl.Execute(&output, input); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		if !strings.Contains(body, "principles/review/test.md") || !strings.Contains(body, "Selected frozen criteria") || strings.Count(body, "## Judgment rules") != 1 {
			t.Fatalf("criteria or common rules are missing: %s", body)
		}
		for _, hidden := range []string{"Repository product body", "Repository rule body", "Hidden specification", "Hidden history path"} {
			if strings.Contains(body, hidden) == detection {
				t.Fatalf("unexpected context visibility for detection=%t: %s", detection, hidden)
			}
		}
	}
	if _, err := fs.Stat(kit.FS(), "templates/aspect.md.tmpl"); err == nil {
		t.Fatal("aspect template is still distributed")
	}
}
