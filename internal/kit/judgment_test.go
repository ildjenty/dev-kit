package kit_test

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"text/template"

	"github.com/ildjenty/dev-kit/internal/kit"
)

func TestJudgmentCriteriaAndContextInputs(t *testing.T) {
	data, err := fs.ReadFile(kit.FS(), "templates/brief/judgment.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("judgment").Option("missingkey=error").Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{
		"InstructionID": "instruction-1", "InstructionIDJSON": `"instruction-1"`,
		"Content": "Anonymous target", "OutputSchema": "Output contract",
		"SupportingSchemas": []map[string]string{},
		"Criteria":          []map[string]string{{"Path": "principles/review/test.md", "Content": "Selected frozen criteria"}},
		"Repository": map[string]any{
			"Requirements": map[string]any{"Path": "requirements.md", "Exists": true, "Content": "Repository product body"},
			"Structure":    map[string]any{"Path": "structure.json", "Exists": false},
			"Rules":        map[string]string{"Path": "rules.md", "Content": "Repository rule body"},
			"LayoutGuides": []map[string]string{},
		},
		"Norms":        []map[string]string{{"Path": "principles/working.md", "Content": "Inline norm body"}},
		"NormPaths":    []map[string]string{{"Path": "history", "SnapshotPath": "Fixed history path"}},
		"Instructions": []string{}, "PreJudgmentInstructions": []string{}, "PreviousOutputs": []map[string]string{},
		"CurrentMaterial": "", "WorkHistoryPath": "Fixed history index", "Delegations": []map[string]string{},
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, input); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	if !strings.Contains(body, "principles/review/test.md") || !strings.Contains(body, "Selected frozen criteria") || strings.Count(body, "## Judgment rules") != 1 {
		t.Fatalf("criteria or common rules are missing: %s", body)
	}
	for _, context := range []string{"Repository product body", "Repository rule body", "Inline norm body", "Fixed history path", "Fixed history index"} {
		if !strings.Contains(body, context) {
			t.Fatalf("judgment context is missing %q: %s", context, body)
		}
	}
	if _, err := fs.Stat(kit.FS(), "templates/aspect.md.tmpl"); err == nil {
		t.Fatal("aspect template is still distributed")
	}
}
