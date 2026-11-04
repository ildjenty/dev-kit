package plan

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/assets"
	"github.com/ildjenty/dev-kit/internal/form"
)

func TestFlowPlanRoundTripAndClone(t *testing.T) {
	v := validator(t)
	pattern, hash := loadTemplate(t, v, "docs")
	p := newTestPlan(pattern, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "Documentation matches the contract"}})
	bindChecks(&p)
	s := &p.Stages[0]
	s.Definition = "edit"
	s.UnitReport = &Selection{Source: "templates", Select: "reports/implement.json"}
	s.HumanPrompt = &Selection{Source: "templates", Select: "review.md"}
	s.Delegations = []Selection{{Source: "rules", Select: ""}}
	s.Outputs = []string{}
	s.History = HistoryNone
	s.Instructions = []string{"Preserve terminology"}
	s.Split = true
	s.Units = []Unit{{ID: "guide", Description: "Revise guide", Scope: "guide.md", Instructions: []string{"Check examples"}, Outputs: []string{"scope"}}}
	s.Checks[0].Instructions = []string{"Verify examples"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.ParsePlan(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) || got.Stages[0].Outputs == nil {
		t.Fatal("flow fields did not round trip")
	}
	next, _ := Replan(p, p)
	stage, _ := next.Stage("edit")
	stage.Report.Select = "changed"
	stage.UnitReport.Select = "changed"
	stage.HumanPrompt.Select = "changed"
	stage.Delegations[0].Source = "changed"
	stage.Instructions[0] = "changed"
	stage.Units[0].Instructions[0] = "changed"
	stage.Units[0].Outputs[0] = "changed"
	stage.Checks[0].Instructions[0] = "changed"
	if !reflect.DeepEqual(p, got) || next.Stages[0].Report.Select != "reports/implement.json" {
		t.Fatal("clone retained mutable fields")
	}
	required := p.Stages[0].RequiredChecks()
	required[0].Instructions[0] = "changed"
	if !reflect.DeepEqual(p, got) {
		t.Fatal("required checks retained mutable fields")
	}
	if (Stage{}).WorkerHistory() != HistoryStageRun || (CheckDefinition{}).JudgmentHistory() != HistoryCheck {
		t.Fatal("incorrect default history")
	}
}

func TestFlowSchemaRestrictions(t *testing.T) {
	v := validator(t)
	pattern, hash := loadTemplate(t, v, "docs")
	for _, check := range []bool{false, true} {
		candidate := pattern
		candidate.Stages = []Stage{cloneStage(pattern.Stages[0]), cloneStage(pattern.Stages[1])}
		if check {
			candidate.Stages[0].Checks[0].Instructions = []string{"Task instruction"}
		} else {
			candidate.Stages[0].Instructions = []string{"Task instruction"}
		}
		if err := v.ValidateTemplate(candidate); err == nil {
			t.Fatal("pattern accepted task instructions")
		}
	}
	p := newTestPlan(pattern, hash, "Revise documentation", []CompletionCondition{{"C1", "Correct guide"}})
	bindChecks(&p)
	for _, change := range []func(*Stage){
		func(s *Stage) { s.History = "unknown" },
		func(s *Stage) { s.Report = &Selection{Source: "bad/name", Select: "schema"} },
		func(s *Stage) { s.UnitReport = &Selection{Source: "", Select: "schema"} },
		func(s *Stage) { s.HumanPrompt = &Selection{Source: "", Select: "prompt"} },
		func(s *Stage) { s.Delegations = []Selection{{Source: "", Select: ""}} },
		func(s *Stage) { s.Outputs = []string{"scope", "scope"} },
	} {
		candidate := clonePlan(p)
		change(&candidate.Stages[0])
		if err := v.ValidatePlan(candidate); err == nil {
			t.Fatal("accepted invalid flow contract")
		}
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"runner", "criteria", "examples", "history", "schemas", "outputs", "brief_by_planner"} {
		body := strings.Replace(string(data), `"aspect":"wording"`, `"aspect":"wording","`+field+`":null`, 1)
		if _, err := v.ParsePlan([]byte(body)); err == nil {
			t.Fatalf("accepted check configuration in a plan: %s", field)
		}
	}
	old := strings.Replace(string(data), `"pattern":`, `"template":`, 1)
	if _, err := v.ParsePlan([]byte(old)); err == nil {
		t.Fatal("accepted removed template reference")
	}
}

func TestBundledReports(t *testing.T) {
	v := validator(t)
	for _, name := range []string{"default", "fix", "docs"} {
		pattern, _ := loadTemplate(t, v, name)
		for _, stage := range append(slices.Clone(pattern.Stages), pattern.OptionalStages...) {
			if stage.Report == nil || stage.Report.Source != "templates" {
				t.Fatalf("%s/%s has no report schema", name, stage.ID)
			}
			body, err := fs.ReadFile(assets.FS(), "templates/"+stage.Report.Select)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := form.CompileSchema(body)
			if err != nil {
				t.Fatal(err)
			}
			if schema.Validate([]byte(`{}`)) == nil || schema.Validate([]byte(`[]`)) == nil {
				t.Fatalf("%s accepts missing report", stage.Report.Select)
			}
			if stage.ID == "retrospective" && stage.WorkerHistory() != HistoryAll {
				t.Fatal("retrospective lacks full history")
			}
		}
	}
}
