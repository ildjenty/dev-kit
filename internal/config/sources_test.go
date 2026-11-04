package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSourceDefaultsAndOverrides(t *testing.T) {
	c, err := Parse([]byte("kit_version = 'v2'\n[sources.principles]\npath = '/vault/principles'\nwritable = false\n[sources.requirements]\nnorm = true\n[sources.extra]\nadapter = 'files'\npath = 'extra'"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Sources) != 9 || c.Sources["principles"].Path != "/vault/principles" || c.Sources["principles"].CanWrite() || !c.Sources["principles"].Norm || !c.Sources["requirements"].Norm {
		t.Fatalf("source defaults or overrides differ: %+v", c)
	}
	if !c.Sources["extra"].CanWrite() || c.Sources["extra"].Norm || c.Sources["extra"].Adapter != "files" {
		t.Fatalf("custom source defaults differ: %+v", c.Sources["extra"])
	}
	copy := c.KnowledgeSources()
	*copy["principles"].Writable = true
	delete(copy, "rules")
	if c.Sources["principles"].CanWrite() || len(c.KnowledgeSources()) != 9 {
		t.Fatal("source accessor modified stored settings")
	}
	if err := c.ValidateImprovementDestination("principles"); err == nil {
		t.Fatal("accepted a read-only improvement destination")
	}
	if err := c.ValidateImprovementDestination("extra"); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidSources(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"[paths]\nrules = 'rules.md'", "unknown configuration key"},
		{"[sources.'bad/name']\npath = 'extra'", "source name"},
		{"[sources.extra]\nadapter = 'command'\npath = 'extra'", "reserved"},
		{"[sources.extra]\nadapter = 'unknown'\npath = 'extra'", "adapter"},
		{"[sources.extra]\npath = ''", "path"},
		{"[sources.extra]", "path"},
		{"[sources.extra]\npath = '.devkit/principles/nested'", "overlap"},
		{"[sources.extra]\npath = '.devkit/requirements.md'", "overlap"},
		{"[sources.extra]\npath = '.devkit'", "overlap"},
		{"[sources.extra]\npath = '/vault'\n[sources.other]\npath = '/vault/nested'", "overlap"},
		{"[plan_check]\ncriteria = [{source = 'missing', select = ''}]", "unknown source"},
		{"[plan_check]\ncriteria = [{source = 'requirements', select = ''}]", "norm source"},
		{"[sources.principles]\nnorm = false", "norm source"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			_, err := Parse([]byte("kit_version = 'v2'\n" + tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want %s", err, tc.want)
			}
		})
	}
	for _, value := range []string{".", "C:/norms", `norms\rules`, "../norms", "norms/../rules", "norms//rules", "./norms", "norms/", ".devkit/run", ".devkit/run/norms"} {
		_, err := Parse([]byte(fmt.Sprintf("kit_version = 'v2'\n[sources.extra]\npath = '%s'", value)))
		if err == nil {
			t.Fatalf("accepted invalid source path: %q", value)
		}
	}
}

func TestPlanCheckSelections(t *testing.T) {
	c, err := Parse([]byte("kit_version = 'v2'\n[plan_check]\ncriteria = [{source = 'rules', select = ''}]"))
	if err != nil {
		t.Fatal(err)
	}
	want := PlanCheck{Inputs: effectiveInputs(nil), Aspect: "plan", Runner: "judge", Criteria: []Selection{{Source: "rules", Select: ""}}, Schemas: []string{"plan", "plan-template", "finding"}}
	if !reflect.DeepEqual(c.PlanCheckSettings(), want) {
		t.Fatalf("plan selections differ: %+v", c.PlanCheckSettings())
	}
	copy := c.PlanCheckSettings()
	copy.Criteria[0].Source = "changed"
	copy.Schemas[0] = "changed"
	if !reflect.DeepEqual(c.PlanCheck, want) {
		t.Fatal("plan check accessor modified stored selections")
	}
}

func TestDefaultSources(t *testing.T) {
	c, err := Parse([]byte("kit_version = 'v2'"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		norm       bool
	}{
		{"principles", ".devkit/principles", true},
		{"stages", ".devkit/stages", true},
		{"plans", ".devkit/plans", true},
		{"templates", ".devkit/templates", true},
		{"rules", ".devkit/rules.md", true},
		{"git", ".devkit/git.md", true},
		{"requirements", ".devkit/requirements.md", false},
		{"structure", ".devkit/structure.json", false},
	} {
		got := c.Sources[tc.name]
		if got.Adapter != "files" || got.Path != tc.path || got.Norm != tc.norm || !got.CanWrite() {
			t.Fatalf("default source %s = %+v", tc.name, got)
		}
	}
	if len(c.Sources) != 8 {
		t.Fatalf("unexpected sources: %+v", c)
	}
}

func TestResolvedSourcePaths(t *testing.T) {
	root := t.TempDir()
	for _, location := range []string{filepath.Join(root, ".devkit/principles/review"), filepath.Join(root, ".devkit/run/knowledge")} {
		c, err := Parse([]byte(fmt.Sprintf("kit_version = 'v2'\n[sources.extra]\npath = %q", location)))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.ValidateSourcePaths(root); err == nil {
			t.Fatalf("accepted overlapping or runtime source: %q", location)
		}
	}
	c, err := Parse([]byte("kit_version = 'v2'"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ValidateSourcePaths(root); err != nil {
		t.Fatal(err)
	}
}
