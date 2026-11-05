package plan

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestPlanApprovalPolicies(t *testing.T) {
	template, hash := loadTemplate(t, validator(t), "docs")
	approved := newTestPlan(template, hash, "Implement behavior", []CompletionCondition{
		{ID: "C1", Description: "First condition"},
		{ID: "C2", Description: "Second condition"},
	})
	for _, tc := range []struct {
		name     string
		policy   string
		previous *Plan
		change   func(*Plan)
		want     bool
	}{
		{"first requirements plan", "requirements", nil, func(*Plan) {}, true},
		{"same requirements", "requirements", &approved, func(*Plan) {}, false},
		{"changed ID", "requirements", &approved, func(p *Plan) { p.CompletionConditions[0].ID = "C3" }, true},
		{"changed text", "requirements", &approved, func(p *Plan) { p.CompletionConditions[0].Description = "Changed condition" }, true},
		{"removed condition", "requirements", &approved, func(p *Plan) { p.CompletionConditions = p.CompletionConditions[:1] }, true},
		{"added condition", "requirements", &approved, func(p *Plan) {
			p.CompletionConditions = append(p.CompletionConditions, CompletionCondition{ID: "C3", Description: "Third condition"})
		}, true},
		{"reordered conditions", "requirements", &approved, func(p *Plan) { slices.Reverse(p.CompletionConditions) }, false},
		{"changed task", "requirements", &approved, func(p *Plan) { p.Task = "Replan behavior" }, false},
		{"open questions", "requirements", &approved, func(p *Plan) { p.OpenQuestions = []string{"Clarify wording"} }, true},
		{"deviation open questions", "deviation", &approved, func(p *Plan) { p.OpenQuestions = []string{"Clarify wording"} }, true},
		{"structure under requirements", "requirements", &approved, func(p *Plan) { p.Stages[0].Gate.Required = true }, false},
		{"external operation", "requirements", &approved, func(p *Plan) { p.ExternalOperations = []ExternalOperation{{Kind: "publish"}} }, true},
		{"always first", "always", nil, func(*Plan) {}, true},
		{"always repeated", "always", &approved, func(*Plan) {}, true},
		{"deviation first", "deviation", nil, func(*Plan) {}, true},
		{"deviation changed text", "deviation", &approved, func(p *Plan) { p.CompletionConditions[0].Description = "Changed condition" }, true},
		{"deviation changed ID", "deviation", &approved, func(p *Plan) { p.CompletionConditions[0].ID = "C3" }, true},
		{"deviation same plan", "deviation", &approved, func(*Plan) {}, false},
		{"deviation reordered conditions", "deviation", &approved, func(p *Plan) { slices.Reverse(p.CompletionConditions) }, false},
		{"deviation approved gate", "deviation", &approved, func(p *Plan) { p.Stages[0].Gate.Required = true }, true},
		{"deviation external", "deviation", nil, func(p *Plan) { p.ExternalOperations = []ExternalOperation{{Kind: "publish"}} }, true},
		{"deviation gate", "deviation", nil, func(p *Plan) { p.Stages[0].Gate.Required = true }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := clonePlan(approved)
			candidate.Version++
			tc.change(&candidate)
			if got := candidate.RequiresApproval(tc.policy, tc.previous); got != tc.want {
				t.Fatalf("approval required: %v; want: %v", got, tc.want)
			}
		})
	}
}

func TestReplanUsesLastHumanApprovedRequirements(t *testing.T) {
	template, hash := loadTemplate(t, validator(t), "docs")
	approved := newTestPlan(template, hash, "Implement behavior", []CompletionCondition{{ID: "C1", Description: "Approved condition"}})
	intermediate, _ := Replan(approved, approved)
	intermediate.CompletionConditions[0].Description = "Unapproved condition"
	latest, _ := Replan(intermediate, approved)
	versions := []VersionEvaluation{
		{Plan: latest, Judgment: PlanPass, Origin: BackProposal},
		{Plan: approved, Judgment: PlanPass, Origin: InitialProposal, Approved: true},
		{Plan: intermediate, Judgment: PlanPass, Origin: HumanProposal},
	}
	if got, ok := ExecutableVersion(versions, 2, "requirements", nil); !ok || got.Version != latest.Version {
		t.Fatal("did not reuse the last human-approved requirements for a back-triggered replan")
	}
	if _, ok := ExecutableVersion(versions, 2, "always", nil); ok {
		t.Fatal("executed an unapproved version under the always policy")
	}
	latest.CompletionConditions[0].Description = "Unapproved condition"
	versions[0].Plan = latest
	if _, ok := ExecutableVersion(versions, 2, "requirements", nil); ok {
		t.Fatal("used an unapproved intermediate version as the requirements baseline")
	}
}

func TestOpenQuestionsAndRemovedMetaLoopStages(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	p := newTestPlan(template, hash, "Implement behavior", []CompletionCondition{{ID: "C1", Description: "Expected behavior"}})
	bindChecks(&p)
	p.OpenQuestions = []string{"Which input should be accepted?"}
	if err := v.ValidatePlan(p); err != nil {
		t.Fatal(err)
	}
	next, _ := Replan(p, p)
	next.OpenQuestions[0] = "Which output should be produced?"
	if p.OpenQuestions[0] == next.OpenQuestions[0] || !slices.Contains(Difference(p, next).ChangedFields, "open_questions") {
		t.Fatal("open questions were not independently copied and compared")
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"version":1`, `"version":1,"meta_loop_stages":[]`, 1))
	if _, err := v.ParsePlan(data); err == nil {
		t.Fatal("accepted removed meta-loop stage declarations")
	}
	data, err = json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte(`{"meta_loop_stages":[],`), data[1:]...)
	if _, err := v.ParseTemplate(data); err == nil {
		t.Fatal("accepted removed template meta-loop stage declarations")
	}
	// Completion condition coverage remains a judgment responsibility.
	p.CompletionConditions = append(p.CompletionConditions, CompletionCondition{ID: "C2", Description: "Unmapped condition"})
	if err := v.ValidatePlan(p); err != nil {
		t.Fatalf("added pending completion condition coverage validation: %v", err)
	}
}
