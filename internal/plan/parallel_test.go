package plan

import (
	"slices"
	"testing"
)

func TestSplitUnitsValidation(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	template.Stages[0].Split = true
	if err := v.ValidateTemplate(template); err != nil {
		t.Fatal(err)
	}
	p := newTestPlan(template, hash, "Update documentation", []CompletionCondition{{ID: "C1", Description: "Updated documents"}})
	bindChecks(&p)
	p.Stages[0].Units = []Unit{{ID: "one", Description: "First change", Scope: "docs/"}, {ID: "two", Description: "Second change", Scope: "docs/"}}
	if err := v.ValidatePlan(p); err != nil {
		t.Fatalf("scope overlap must be checked by the plan judge: %v", err)
	}
	next, _ := Replan(p, p)
	next.Stages[0].Units[0].Scope = "other/"
	if p.Stages[0].Units[0].Scope != "docs/" || !slices.Contains(Difference(p, next).ChangedStages, p.Stages[0].ID) {
		t.Fatal("units were not cloned or compared independently")
	}
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Stages[0].Units[1].ID = "one" },
		func(p *Plan) {
			p.Stages[1].Split = true
			p.Stages[1].Units = []Unit{{ID: "one", Description: "Duplicate across stages", Scope: "other/"}}
		},
		func(p *Plan) { p.Stages[0].Split = false },
		func(p *Plan) { p.Stages[0].Units[0].Description = "" },
		func(p *Plan) { p.Stages[0].Units[0].Scope = "" },
	} {
		candidate := clonePlan(p)
		change(&candidate)
		if err := v.ValidatePlan(candidate); err == nil {
			t.Fatal("accepted invalid unit data")
		}
	}
	template.Stages[0].Units = p.Stages[0].Units
	if err := v.ValidateTemplate(template); err == nil {
		t.Fatal("accepted task units in a template")
	}
}
