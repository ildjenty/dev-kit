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
