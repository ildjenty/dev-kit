package kit_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestBundledCriteriaSelectTheirRequiredDocuments(t *testing.T) {
	schemas, err := kit.Schema("plan-template")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := kit.Schema("plan")
	if err != nil {
		t.Fatal(err)
	}
	validator, err := plan.NewValidator(schemas, plans)
	if err != nil {
		t.Fatal(err)
	}
	defaults := (config.Config{}).PlanCheckSettings()
	planCriteria := []plan.Selection{}
	for _, selection := range defaults.Criteria {
		planCriteria = append(planCriteria, plan.Selection{Source: selection.Source, Select: selection.Select})
	}
	checks := []plan.Check{{Aspect: defaults.Aspect, Criteria: planCriteria}}
	for _, name := range []string{"default", "fix", "docs"} {
		data, err := fs.ReadFile(kit.FS(), "plans/"+name+".json")
		if err != nil {
			t.Fatal(err)
		}
		template, err := validator.ParseTemplate(data)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(template.PlannerInputs, plan.DefaultInputs()) || !slices.Equal(template.JudgeInputs, plan.DefaultInputs()) {
			t.Fatalf("%s does not express default role inputs as data", name)
		}
		for _, stage := range append(template.Stages, template.OptionalStages...) {
			if !slices.Equal(stage.Inputs, append(plan.DefaultInputs(), plan.Selection{Source: "git"})) {
				t.Fatalf("%s/%s does not express worker inputs as data", name, stage.ID)
			}
			checks = append(checks, stage.Checks...)
		}
	}
	references := regexp.MustCompile("`((?:principles|stages|plans|templates|examples)/[^`]+|rules[.]md)`")
	for _, item := range checks {
		if item.Runner.IsCommand() {
			continue
		}
		review, err := fs.ReadFile(kit.FS(), "principles/review/"+item.Aspect+".md")
		if err != nil {
			t.Fatal(err)
		}
		_, criteria, found := strings.Cut(string(review), "## Criteria\n")
		if !found {
			continue
		}
		criteria, _, _ = strings.Cut(criteria, "\n## ")
		selected := map[string]bool{}
		for _, selection := range item.Criteria {
			selected[selection.Source+"/"+selection.Select] = true
		}
		for _, reference := range references.FindAllStringSubmatch(criteria, -1) {
			document := reference[1]
			if document == "rules.md" {
				document = "rules/"
			}
			if !selected[document] {
				t.Errorf("%s omits required criteria document %s", item.Aspect, reference[1])
			}
		}
	}
}

func TestTemplateExampleCriteriaMatchBundledTemplates(t *testing.T) {
	data, err := os.ReadFile("../../docs/architecture/examples.md")
	if err != nil {
		t.Fatal(err)
	}
	fix, err := fs.ReadFile(kit.FS(), "plans/fix.json")
	if err != nil {
		t.Fatal(err)
	}
	var template plan.PlanTemplate
	if err := json.Unmarshal(fix, &template); err != nil {
		t.Fatal(err)
	}
	checks := map[string]plan.Check{}
	for _, stage := range template.Stages {
		for _, item := range stage.Checks {
			checks[item.Aspect] = item
		}
	}
	blocks := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindAllStringSubmatch(string(data), -1)
	for _, block := range blocks {
		var object any
		if err := json.Unmarshal([]byte(block[1]), &object); err != nil {
			t.Fatal(err)
		}
		compareExampleCriteria(t, object, checks)
	}
}

func compareExampleCriteria(t *testing.T, value any, checks map[string]plan.Check) {
	t.Helper()
	switch value := value.(type) {
	case []any:
		for _, child := range value {
			compareExampleCriteria(t, child, checks)
		}
	case map[string]any:
		if aspect, ok := value["aspect"].(string); ok && (aspect == "diff" || aspect == "harness") {
			body, err := json.Marshal(value["criteria"])
			if err != nil {
				t.Fatal(err)
			}
			var selections []plan.Selection
			if err := json.Unmarshal(body, &selections); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(selections, checks[aspect].Criteria) {
				t.Errorf("example %s criteria differ from the bundled template", aspect)
			}
		}
		for _, child := range value {
			compareExampleCriteria(t, child, checks)
		}
	}
}

func TestDistributedDocsDescribeCurrentSourcesAndStageReading(t *testing.T) {
	err := fs.WalkDir(kit.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := fs.ReadFile(kit.FS(), name)
		if err != nil {
			return err
		}
		for _, stale := range []string{"[paths]", ".devkit/checks/"} {
			if strings.Contains(string(data), stale) {
				t.Errorf("%s still documents %s", name, stale)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(readme), ".devkit/checks/") {
		t.Fatal("README documents the removed checks directory")
	}
	stages, err := fs.ReadFile(kit.FS(), "stages/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stages), "entire stage document body without interpreting its sections") || !strings.Contains(string(stages), "[sources.<name>]") {
		t.Fatal("stage documentation does not match whole-document source selection")
	}
}
