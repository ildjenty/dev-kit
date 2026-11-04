package config

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"text/template"

	"github.com/ildjenty/dev-kit/assets"
)

func TestPlannerTemplateCatalogs(t *testing.T) {
	body, err := fs.ReadFile(assets.FS(), "templates/brief/planner.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("planner").Option("missingkey=error").Parse(string(body))
	if err != nil {
		t.Fatal(err)
	}
	path := func(logical, fixed string) map[string]string {
		return map[string]string{"Path": logical, "SnapshotPath": fixed, "Content": "definition body must remain at its fixed path"}
	}
	data := map[string]any{
		"InstructionID": "instruction-1", "InstructionIDJSON": `"instruction-1"`, "OriginalRequest": "Edit a guide", "Task": "Edit a guide",
		"PatternHash": "hash", "PatternJSON": `{"name":"docs"}`, "Patterns": []map[string]string{path("plans/fix.json", "snap/fix.json")},
		"StageCatalog": "- edit: Document editing", "StageDefinitions": []map[string]string{path("stages/edit.md", "snap/edit.md")},
		"RolesJSON": `[{"id":"worker"}]`, "LoopRulesJSON": `{"last":"retrospective"}`, "ChecksJSON": `[{"aspect":"wording","description":"Review wording"}]`,
		"Repository": map[string]any{"Structure": map[string]string{"Path": ""}, "LayoutGuides": nil, "Rules": map[string]string{"Path": ""}, "Requirements": map[string]string{"Path": ""}},
		"Norms":      nil, "NormPaths": nil, "CurrentPlan": nil, "RemainingPlan": nil, "HumanInstructions": "", "PreviousFindings": nil,
		"PreviousOutputs": nil, "Version": 1, "OutputSchema": "",
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`{"name":"docs"}`, "plans/fix.json: snap/fix.json", "edit: Document editing", "stages/edit.md: snap/edit.md", `"id":"worker"`, "Review wording"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("planner brief omitted %s", expected)
		}
	}
	if strings.Contains(output.String(), "definition body must remain") {
		t.Fatal("planner brief inlined definition bodies")
	}
}
