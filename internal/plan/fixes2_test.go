package plan

import "testing"

func TestDoneTransitionsRejectCycles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		done    map[string]*string
		invalid bool
	}{
		{"self cycle", map[string]*string{"implement": doneTarget("implement")}, true},
		{"two stage cycle", map[string]*string{"implement": doneTarget("retrospective"), "retrospective": doneTarget("implement")}, true},
		{"disconnected cycle", map[string]*string{"implement": nil, "retrospective": doneTarget("retrospective")}, true},
		{"null terminates", map[string]*string{"implement": doneTarget("retrospective"), "retrospective": nil}, false},
		{"replan terminates", map[string]*string{"implement": doneTarget("replan"), "retrospective": doneTarget("implement")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := validator(t)
			template, hash := loadTemplate(t, v, "docs")
			for i := range template.Stages {
				template.Stages[i].Transitions.Done = tc.done[template.Stages[i].ID]
			}
			if err := v.ValidateTemplate(template); (err != nil) != tc.invalid {
				t.Fatalf("template validation: %v; invalid=%v", err, tc.invalid)
			}
			p := NewPlan(template, hash, "Update documentation", []CompletionCondition{{ID: "C1", Description: "Documentation is complete"}})
			bindChecks(&p)
			if err := v.ValidatePlan(p); (err != nil) != tc.invalid {
				t.Fatalf("plan validation: %v; invalid=%v", err, tc.invalid)
			}
		})
	}
}

func doneTarget(id string) *string { return &id }

func TestReplannedPlanMayGoBackToARemovedStage(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "Build a CLI", []CompletionCondition{{"C1", "The CLI works"}})
	bindChecks(&p)
	// After purpose completes, the replanned plan starts at scope and keeps scope's back edge to purpose.
	p.Stages = p.Stages[1:]
	p.StartStage = p.Stages[0].ID
	if back, _ := p.Stages[0].Transitions.Next(Back); back == nil || *back != "purpose" {
		t.Fatalf("test premise: scope must go back to purpose, got %v", back)
	}
	if err := v.ValidatePlan(p); err != nil {
		t.Fatalf("replanned plan rejected: %v", err)
	}
	broken := template
	broken.Stages = append([]Stage(nil), template.Stages[1:]...)
	broken.StartStage = broken.Stages[0].ID
	if err := v.ValidateTemplate(broken); err == nil {
		t.Fatal("template with a back edge to a missing stage was accepted")
	}
}
