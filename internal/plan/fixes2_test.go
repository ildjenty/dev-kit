package plan

import "testing"

func TestDoneTransitionsRejectCycles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		done    map[string]*string
		invalid bool
	}{
		{"self cycle", map[string]*string{"edit": doneTarget("edit")}, true},
		{"two stage cycle", map[string]*string{"edit": doneTarget("retrospective"), "retrospective": doneTarget("edit")}, true},
		{"disconnected cycle", map[string]*string{"edit": nil, "retrospective": doneTarget("retrospective")}, true},
		{"null terminates", map[string]*string{"edit": doneTarget("retrospective"), "retrospective": nil}, false},
		{"replan leaves unreachable stage", map[string]*string{"edit": doneTarget("replan"), "retrospective": doneTarget("edit")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := validator(t)
			template, hash := loadTemplate(t, v, "docs")
			for i := range template.Stages {
				template.Stages[i].Transitions.Done = tc.done[template.Stages[i].ID]
			}
			if err := v.ValidateTemplate(template); (err != nil) != (tc.invalid && tc.name != "replan leaves unreachable stage") {
				t.Fatalf("template validation: %v; invalid=%v", err, tc.invalid)
			}
			p := newTestPlan(template, hash, "Update documentation", []CompletionCondition{{ID: "C1", Description: "Documentation is complete"}})
			p.Stages = template.Stages
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
	for _, tc := range []struct {
		name       string
		start, end int
		back       string
	}{
		// After purpose completes, the replanned plan runs scope through increments and keeps scope's back edge to purpose.
		{"after purpose", 1, 4, "purpose"},
		// After increments completes, the replanned plan starts at the first design and keeps its back edge to increments.
		{"after increments", 4, len(template.Stages), "increments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlan(template, hash, "Build a CLI", []CompletionCondition{{"C1", "The CLI works"}})
			p.Stages = clonePlan(Plan{Stages: template.Stages[tc.start:tc.end]}).Stages
			bindChecks(&p)
			p.StartStage = p.Stages[0].ID
			if back, _ := p.Stages[0].Transitions.Next(Back); back == nil || *back != tc.back {
				t.Fatalf("test premise: %s must go back to %s, got %v", p.StartStage, tc.back, back)
			}
			if err := v.ValidatePlan(p); err != nil {
				t.Fatalf("replanned plan rejected: %v", err)
			}
		})
	}
	broken := template
	broken.Stages = append([]Stage(nil), template.Stages[1:]...)
	broken.StartStage = broken.Stages[0].ID
	if err := v.ValidateTemplate(broken); err == nil {
		t.Fatal("template with a back edge to a missing stage was accepted")
	}
}
