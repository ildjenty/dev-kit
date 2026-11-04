package plan

import (
	"reflect"
	"slices"
	"testing"
)

func TestKnowledgeSelectionRoundTrip(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	template.Stages[0].Inputs = []Selection{{Source: "principles", Select: "writing.md"}}
	if err := v.ValidateTemplate(template); err != nil {
		t.Fatal(err)
	}
	p := NewPlan(template, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "The documentation matches the contract"}})
	bindChecks(&p)
	if err := v.ValidatePlan(p); err != nil {
		t.Fatal(err)
	}
	want := []Selection{{Source: "requirements", Select: ""}, {Source: "rules", Select: ""}, {Source: "principles", Select: "writing.md"}}
	if got := p.Stages[0].WorkerInputs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("worker inputs = %+v; want %+v", got, want)
	}
	if got := (Stage{Inputs: DefaultInputs()}).WorkerInputs(); !reflect.DeepEqual(got, DefaultInputs()) {
		t.Fatalf("default inputs were duplicated: %+v", got)
	}
	next, _ := Replan(p, p)
	next.Stages[0].Inputs[0].Select = "changed.md"
	next.Stages[0].Checks[0].Criteria[0].Select = "changed.md"
	next.Stages[0].Checks[0].Examples.Select = "changed/spec.json"
	if p.Stages[0].Inputs[0].Select != "writing.md" || p.Stages[0].Checks[0].Criteria[0].Select != "review/wording.md" || p.Stages[0].Checks[0].Examples.Select != "wording/spec.json" {
		t.Fatal("replanning changed the original knowledge selections")
	}
	required := p.Stages[0].RequiredChecks()
	required[0].Criteria[0].Select = "changed.md"
	required[0].Examples.Select = "changed/spec.json"
	if !slices.Equal(template.Stages[0].Checks[0].Criteria, p.Stages[0].Checks[0].Criteria) || *template.Stages[0].Checks[0].Examples != *p.Stages[0].Checks[0].Examples {
		t.Fatal("required checks changed the original knowledge selections")
	}
}

func TestSelectionChangesRequireApproval(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	original := NewPlan(template, hash, "Revise documentation", []CompletionCondition{{ID: "C1", Description: "The documentation matches the contract"}})
	bindChecks(&original)
	for _, tc := range []struct {
		name   string
		kind   DeviationKind
		change func(*Check)
	}{
		{"criteria", ChangedCriteria, func(c *Check) { c.Criteria[0].Select = "review/diff.md" }},
		{"criteria removed", ChangedCriteria, func(c *Check) { c.Criteria = nil }},
		{"examples", ChangedExamples, func(c *Check) { c.Examples.Select = "diff/spec.json" }},
		{"examples removed", ChangedExamples, func(c *Check) { c.Examples = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := clonePlan(original)
			tc.change(&p.Stages[0].Checks[0])
			want := []Deviation{{Kind: tc.kind, StageID: "implement", Aspect: "wording", Runner: "devkit-judge"}}
			if got := Deviations(template, p); !reflect.DeepEqual(got, want) {
				t.Fatalf("deviations = %+v; want %+v", got, want)
			}
			if !p.RequiresApproval(template, "deviation", &original) {
				t.Fatal("selection change bypassed approval")
			}
			duplicate := cloneCheck(original.Stages[0].Checks[0])
			p.Stages[0].Checks = append(p.Stages[0].Checks, duplicate)
			if err := v.ValidatePlan(p); err == nil {
				t.Fatal("accepted conflicting knowledge selections for one check")
			}
		})
	}
}

func TestKnowledgeSelectionSchema(t *testing.T) {
	v := validator(t)
	template, _ := loadTemplate(t, v, "docs")
	for _, selection := range []Selection{{Source: "", Select: ""}, {Source: "bad/name", Select: ""}} {
		candidate := template
		candidate.Stages = []Stage{cloneStage(template.Stages[0]), cloneStage(template.Stages[1])}
		candidate.Stages[0].Checks[0].Criteria = []Selection{selection}
		if err := v.ValidateTemplate(candidate); err == nil {
			t.Fatalf("accepted invalid criteria selection: %+v", selection)
		}
		candidate.Stages[0].Checks[0].Criteria = template.Stages[0].Checks[0].Criteria
		candidate.Stages[0].Checks[0].Examples = &selection
		if err := v.ValidateTemplate(candidate); err == nil {
			t.Fatalf("accepted invalid examples selection: %+v", selection)
		}
		candidate.Stages[0].Checks[0].Examples = template.Stages[0].Checks[0].Examples
		candidate.Stages[0].Inputs = []Selection{selection}
		if err := v.ValidateTemplate(candidate); err == nil {
			t.Fatalf("accepted invalid stage input: %+v", selection)
		}
	}
	command := cloneStage(template.Stages[0])
	command.Checks[0].Aspect = "command:test"
	command.Checks[0].Runner = "command:test"
	template.Stages[0] = command
	if err := v.ValidateTemplate(template); err == nil {
		t.Fatal("command check accepted criteria and examples")
	}
}
