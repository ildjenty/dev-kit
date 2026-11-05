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

// bundledCatalog returns the default [checks.<aspect>] catalog written by init and the default
// plan check, keyed by aspect.
func bundledCatalog(t *testing.T) map[string]config.CheckDefinition {
	t.Helper()
	fragment, err := kit.HostFragment("roles.toml")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := config.Parse(append([]byte("kit_version = \"v3\"\n"), fragment...))
	if err != nil {
		t.Fatal(err)
	}
	catalog := map[string]config.CheckDefinition{}
	for aspect, item := range settings.Checks {
		catalog[aspect] = item
	}
	defaults := settings.PlanCheckSettings()
	catalog[defaults.Aspect] = config.CheckDefinition{Runner: defaults.Runner, Criteria: defaults.Criteria}
	return catalog
}

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
	catalog := bundledCatalog(t)
	for _, name := range []string{"default", "fix", "docs"} {
		data, err := fs.ReadFile(kit.FS(), "plans/"+name+".json")
		if err != nil {
			t.Fatal(err)
		}
		template, err := validator.ParseTemplate(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, stage := range append(template.Stages, template.OptionalStages...) {
			if !slices.Equal(stage.Inputs, append(plan.RepositoryInputs(), plan.Selection{Source: "git"}, plan.Selection{Source: "authority"})) {
				t.Fatalf("%s/%s does not express worker inputs as data", name, stage.ID)
			}
			for _, item := range stage.Checks {
				if _, ok := catalog[item.Aspect]; !ok {
					t.Errorf("%s/%s selects unconfigured check %s", name, stage.ID, item.Aspect)
				}
			}
		}
	}
	references := regexp.MustCompile("`((?:principles|stages|plans|templates)/[^`]+|rules[.]md)`")
	for aspect, item := range catalog {
		if strings.HasPrefix(item.Runner, "command:") {
			continue
		}
		review, err := fs.ReadFile(kit.FS(), "principles/review/"+aspect+".md")
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
				t.Errorf("%s omits required criteria document %s", aspect, reference[1])
			}
		}
	}
}

func TestTemplateExampleCriteriaMatchBundledTemplates(t *testing.T) {
	data, err := os.ReadFile("../../docs/architecture/examples.md")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][]plan.Selection{}
	for aspect, item := range bundledCatalog(t) {
		for _, selection := range item.Criteria {
			checks[aspect] = append(checks[aspect], plan.Selection{Source: selection.Source, Select: selection.Select})
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

func compareExampleCriteria(t *testing.T, value any, checks map[string][]plan.Selection) {
	t.Helper()
	switch value := value.(type) {
	case []any:
		for _, child := range value {
			compareExampleCriteria(t, child, checks)
		}
	case map[string]any:
		if aspect, ok := value["aspect"].(string); ok && (aspect == "diff" || aspect == "harness") && value["criteria"] != nil {
			body, err := json.Marshal(value["criteria"])
			if err != nil {
				t.Fatal(err)
			}
			var selections []plan.Selection
			if err := json.Unmarshal(body, &selections); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(selections, checks[aspect]) {
				t.Errorf("example %s criteria differ from the bundled check catalog", aspect)
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
