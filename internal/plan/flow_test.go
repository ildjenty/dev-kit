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
	template, hash := loadTemplate(t, v, "docs")
	p := NewPlan(template, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "Documentation matches the contract"}})
	bindChecks(&p)
	s := &p.Stages[0]
	s.UnitReport = &Selection{Source: "templates", Select: "reports/implement.json"}
	s.HumanPrompt = &Selection{Source: "templates", Select: "review.md"}
	s.Delegations = []Selection{{Source: "rules", Select: ""}}
	s.Outputs = []string{}
	s.History = HistoryNone
	s.Instructions = []string{"Preserve existing terminology"}
	s.Split = true
	s.Units = []Unit{{ID: "guide", Description: "Revise the guide", Scope: "guide.md", Instructions: []string{"Check examples"}, Outputs: []string{"scope"}}}
	s.Checks[0].History = HistoryAll
	s.Checks[0].Outputs = []string{}
	s.Checks[0].Schemas = []string{"plan", "plan-template"}
	s.Checks[0].BriefByPlanner = true
	s.Checks[0].Instructions = []string{"Verify the examples"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.ParsePlan(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) || got.Stages[0].Outputs == nil || got.Stages[0].Checks[0].Outputs == nil {
		t.Fatalf("flow fields did not round trip: %+v", got)
	}
	next, _ := Replan(p, p)
	stage, _ := next.Stage("implement")
	stage.Report.Select = "changed"
	stage.UnitReport.Select = "changed"
	stage.HumanPrompt.Select = "changed"
	stage.Delegations[0].Source = "changed"
	stage.Instructions[0] = "changed"
	stage.Units[0].Instructions[0] = "changed"
	stage.Units[0].Outputs[0] = "changed"
	stage.Checks[0].Schemas[0] = "changed"
	stage.Checks[0].Instructions[0] = "changed"
	if !reflect.DeepEqual(p, got) || next.Stages[0].Report.Select != "reports/implement.json" {
		t.Fatal("cloning retained mutable flow fields")
	}
	required := p.Stages[0].RequiredChecks()
	required[0].Schemas[0] = "changed"
	required[0].Instructions[0] = "changed"
	if !reflect.DeepEqual(p, got) {
		t.Fatal("required check clone retained mutable flow fields")
	}
	if (Stage{}).WorkerHistory() != HistoryStageRun || (Check{}).JudgmentHistory() != HistoryCheck {
		t.Fatal("incorrect default history scope")
	}
}

func TestFlowDeviations(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	original := NewPlan(template, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "Documentation matches the contract"}})
	bindChecks(&original)
	for _, tc := range []struct {
		name   string
		kind   DeviationKind
		check  bool
		change func(*Stage)
	}{
		{"report", ChangedReport, false, func(s *Stage) { s.Report = nil }},
		{"unit report", ChangedUnitReport, false, func(s *Stage) { s.UnitReport = s.Report }},
		{"human prompt", ChangedHumanPrompt, false, func(s *Stage) { s.HumanPrompt = s.Report }},
		{"delegations", ChangedDelegations, false, func(s *Stage) { s.Delegations = []Selection{{Source: "rules", Select: ""}} }},
		{"stage outputs", ChangedOutputs, false, func(s *Stage) { s.Outputs = []string{} }},
		{"stage history", ChangedHistory, false, func(s *Stage) { s.History = HistoryNone }},
		{"check history", ChangedHistory, true, func(s *Stage) { s.Checks[0].History = HistoryAll }},
		{"schemas", ChangedSchemas, true, func(s *Stage) { s.Checks[0].Schemas = []string{"plan"} }},
		{"check outputs", ChangedOutputs, true, func(s *Stage) { s.Checks[0].Outputs = []string{} }},
		{"planner guidance", ChangedBriefByPlanner, true, func(s *Stage) { s.Checks[0].BriefByPlanner = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := clonePlan(original)
			tc.change(&p.Stages[0])
			want := Deviation{Kind: tc.kind, StageID: "implement"}
			if tc.check {
				want.Aspect, want.Runner = "wording", "judge"
			}
			if got := Deviations(template, p); !reflect.DeepEqual(got, []Deviation{want}) {
				t.Fatalf("deviations = %+v; want %+v", got, want)
			}
			if !p.RequiresApproval(template, "deviation", &original) {
				t.Fatal("template flow change bypassed approval")
			}
		})
	}
	p := clonePlan(original)
	p.Stages[0].Instructions = []string{"Preserve wording"}
	p.Stages[0].Checks[0].Instructions = []string{"Check terminology"}
	p.Stages[0].History = HistoryStageRun
	p.Stages[0].Checks[0].History = HistoryCheck
	if got := Deviations(template, p); len(got) != 0 {
		t.Fatalf("plan instructions or explicit default history are deviations: %+v", got)
	}
}

func TestFlowSchemaRestrictions(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	for _, check := range []bool{false, true} {
		candidate := template
		candidate.Stages = []Stage{cloneStage(template.Stages[0]), cloneStage(template.Stages[1])}
		if check {
			candidate.Stages[0].Checks[0].Instructions = []string{"Task-specific instruction"}
		} else {
			candidate.Stages[0].Instructions = []string{"Task-specific instruction"}
		}
		if err := v.ValidateTemplate(candidate); err == nil {
			t.Fatal("template accepted task-specific instructions")
		}
	}
	p := NewPlan(template, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "Documentation matches the contract"}})
	bindChecks(&p)
	for _, change := range []func(*Stage){
		func(s *Stage) { s.History = "unknown" },
		func(s *Stage) { s.Checks[0].History = "unknown" },
		func(s *Stage) { s.Report = &Selection{Source: "bad/name", Select: "schema.json"} },
		func(s *Stage) { s.UnitReport = &Selection{Source: "", Select: "schema.json"} },
		func(s *Stage) { s.HumanPrompt = &Selection{Source: "", Select: "prompt.md"} },
		func(s *Stage) { s.Delegations = []Selection{{Source: "", Select: ""}} },
		func(s *Stage) { s.Outputs = []string{"scope", "scope"} },
		func(s *Stage) { s.Checks[0].Schemas = []string{"../plan"} },
	} {
		candidate := clonePlan(p)
		change(&candidate.Stages[0])
		if err := v.ValidatePlan(candidate); err == nil {
			t.Fatal("accepted invalid flow contract")
		}
	}
	p.Stages[0].Split = true
	p.Stages[0].Units = []Unit{{ID: "guide", Description: "Revise guide", Scope: "guide.md"}}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"scope":"guide.md"`, `"scope":"guide.md","after":["other"]`, 1))
	if _, err := v.ParsePlan(data); err == nil {
		t.Fatal("accepted unit dependencies")
	}
}

func TestFlowCheckIdentityConflicts(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	original := NewPlan(template, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "Documentation matches the contract"}})
	bindChecks(&original)
	for _, change := range []func(*Check){
		func(c *Check) { c.History = HistoryNone },
		func(c *Check) { c.Schemas = []string{"plan"} },
		func(c *Check) { c.Outputs = []string{} },
		func(c *Check) { c.BriefByPlanner = true },
		func(c *Check) { c.Instructions = []string{"Check terminology"} },
	} {
		p := clonePlan(original)
		duplicate := cloneCheck(p.Stages[0].Checks[0])
		change(&duplicate)
		p.Stages[0].Checks = append(p.Stages[0].Checks, duplicate)
		if err := v.ValidatePlan(p); err == nil {
			t.Fatal("accepted conflicting flow fields for one aspect and runner")
		}
		p.Stages[0].Checks[1].Runner = "other-judge"
		if err := v.ValidatePlan(p); err != nil {
			t.Fatalf("different runners shared a check identity: %v", err)
		}
	}
}

func TestBundledReports(t *testing.T) {
	v := validator(t)
	for _, name := range []string{"default", "fix", "docs"} {
		template, _ := loadTemplate(t, v, name)
		for _, stage := range append(slices.Clone(template.Stages), template.OptionalStages...) {
			if stage.Report == nil || stage.Report.Source != "templates" {
				t.Fatalf("%s/%s has no norm-owned report schema", name, stage.ID)
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
				t.Fatalf("%s accepts a missing report", stage.Report.Select)
			}
			if stage.ID == "retrospective" && stage.WorkerHistory() != HistoryAll {
				t.Fatal("retrospective lacks full history")
			}
			for _, check := range stage.Checks {
				if check.BriefByPlanner || check.Aspect == "harness" && check.JudgmentHistory() != HistoryAll {
					t.Fatalf("unexpected default check context: %+v", check)
				}
			}
		}
	}
}
