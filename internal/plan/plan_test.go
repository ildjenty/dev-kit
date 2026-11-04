package plan

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/assets"
)

func validator(t *testing.T) *Validator {
	t.Helper()
	read := func(path string) []byte {
		t.Helper()
		data, err := fs.ReadFile(assets.FS(), path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	v, err := NewValidator(read(TemplateSchemaPath), read(PlanSchemaPath))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func loadTemplate(t *testing.T, v *Validator, name string) (PlanTemplate, string) {
	t.Helper()
	data, err := fs.ReadFile(assets.FS(), "plans/"+name+".json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := v.ParseTemplate(data)
	if err != nil {
		t.Fatal(err)
	}
	return template, ContentHash(data)
}

func bindChecks(p *Plan) {
	for i := range p.Stages {
		for j := range p.Stages[i].Checks {
			p.Stages[i].Checks[j].CompletionConditionIDs = []string{"C1"}
			p.Stages[i].Checks[j].Target = "returned tree"
		}
	}
}

func TestSchemaAndReferences(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := newTestPlan(template, hash, "Fix wording", []CompletionCondition{{"C1", "Wording is correct"}})
	if err := v.ValidatePlan(p); err == nil {
		t.Fatal("accepted a plan without check targets and completion condition mappings")
	}
	bindChecks(&p)
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Stages[0].Checks[0].CompletionConditionIDs = []string{"missing"} },
		func(p *Plan) { p.Stages[0].Definition = ""; p.Stages[0].Transitions.Ask = "other" },
		func(p *Plan) { p.Stages = append(p.Stages, p.Stages[0]) },
		func(p *Plan) { p.Stages[0].Limits.MaxRounds = 0 },
		func(p *Plan) { p.Stages[0].Assignee = "" },
	} {
		bad := clonePlan(p)
		change(&bad)
		if err := v.ValidatePlan(bad); err == nil {
			t.Fatal("accepted an invalid plan")
		}
	}
	missingBack := clonePlan(p)
	missingBack.Stages[0].Transitions.Back = "missing"
	if err := ValidateBackTargets(missingBack, nil); err == nil {
		t.Fatal("accepted a back transition to a stage absent from the plan and the template")
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.ParsePlan(append(data, []byte(" {}")...)); err == nil {
		t.Fatal("accepted an extra trailing JSON value")
	}
	if _, err := v.ParsePlan([]byte(strings.Replace(string(data), `"version":1`, `"version":1,"unknown":true`, 1))); err == nil {
		t.Fatal("accepted a key absent from the schema")
	}
}

func TestTemplateChecksExcludeTaskFields(t *testing.T) {
	v := validator(t)
	for _, optional := range []bool{false, true} {
		for _, field := range []string{"completion_condition_ids", "target"} {
			name := "default/" + field
			if optional {
				name = "optional/" + field
			}
			t.Run(name, func(t *testing.T) {
				template, _ := loadTemplate(t, v, "default")
				stage := &template.Stages[0]
				if optional {
					stage = &template.OptionalStages[0]
					stage.Checks = []Check{{Aspect: "wording", Required: true}}
				}
				if field == "completion_condition_ids" {
					stage.Checks[0].CompletionConditionIDs = []string{"C1"}
				} else {
					stage.Checks[0].Target = "returned tree"
				}
				if err := v.ValidateTemplate(template); err == nil {
					t.Fatal("accepted task-specific check fields in a template")
				}
			})
		}
	}
}

func TestCheckMappingsWithSharedAspect(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := newTestPlan(template, hash, "Fix behavior", []CompletionCondition{
		{"C1", "First target is correct"},
		{"C2", "Second target is correct"},
	})
	bindChecks(&p)
	for _, tc := range []struct {
		name   string
		change func(*Check)
		valid  bool
	}{
		{"different target", func(c *Check) { c.Target = "another target" }, false},
		{"different conditions", func(c *Check) { c.CompletionConditionIDs = []string{"C2"} }, false},
		{"identical mapping", func(c *Check) {}, true},
		{"different aspect", func(c *Check) {
			c.Aspect = "behavior"
			c.Target = "another target"
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := clonePlan(p)
			duplicate := cloneCheck(candidate.Stages[0].Checks[0])
			tc.change(&duplicate)
			candidate.Stages[0].Checks = append(candidate.Stages[0].Checks, duplicate)
			err := v.ValidatePlan(candidate)
			if tc.valid && err != nil {
				t.Fatal(err)
			}
			if !tc.valid && (err == nil || !strings.Contains(err.Error(), "conflicting check mappings")) {
				t.Fatalf("expected conflicting check mappings, got %v", err)
			}
		})
	}
}

func TestWithdrawnHumanProposalAfterReplanning(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := newTestPlan(template, hash, "Fix behavior", []CompletionCondition{{"C1", "Works as expected"}})
	bindChecks(&p)
	replan, _ := Replan(p, p)
	proposal, _ := Replan(replan, replan)
	versions := []VersionEvaluation{
		{Plan: p, Judgment: PlanPass, Origin: InitialProposal},
		{Plan: replan, Judgment: PlanPass, Origin: BackProposal, Approved: true},
		{Plan: proposal, Origin: HumanProposal, Withdrawn: true},
	}
	if got, ok := ExecutableVersion(versions, 1, "requirements", nil); !ok || got.Version != 2 {
		t.Fatal("could not select an approved replan after a voluntary human withdrawal")
	}
	if _, ok := ExecutableVersion(versions, 2, "requirements", nil); ok {
		t.Fatal("selected an old version after a voluntary human withdrawal")
	}
}

func TestVersionsAndExternalApproval(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := newTestPlan(template, hash, "Fix", []CompletionCondition{{"C1", "Correct"}})
	bindChecks(&p)
	next, diff := Replan(p, p)
	if next.Version != 2 || len(diff.ChangedFields) != 0 || len(diff.ChangedStages) != 0 {
		t.Fatalf("replanning: %+v, %+v", next, diff)
	}
	next.Stages[0].Checks[0].CompletionConditionIDs[0] = "changed"
	if p.Stages[0].Checks[0].CompletionConditionIDs[0] != "C1" {
		t.Fatal("modified the old plan")
	}
	old := VersionEvaluation{Plan: p, Judgment: PlanPass, Origin: InitialProposal, Approved: true}
	proposal := VersionEvaluation{Plan: next, Origin: HumanProposal}
	if _, ok := ExecutableVersion([]VersionEvaluation{old, proposal}, 0, "requirements", nil); ok {
		t.Fatal("selected an old plan while a proposal was pending")
	}
	proposal.Withdrawn = true
	if got, ok := ExecutableVersion([]VersionEvaluation{proposal, old}, 0, "requirements", nil); !ok || got.Version != 1 {
		t.Fatal("could not return to the old plan after a voluntary human withdrawal")
	}
	proposal.Origin = BackProposal
	if _, ok := ExecutableVersion([]VersionEvaluation{proposal, old}, 1, "requirements", nil); ok {
		t.Fatal("selected an old plan after a replanning withdrawal")
	}
	proposal.Withdrawn = false
	proposal.Judgment = PlanPass
	if proposal.Executable("requirements", nil) {
		t.Fatal("made a back-triggered replan executable without human approval")
	}
	proposal.Approved = true
	if got, ok := ExecutableVersion([]VersionEvaluation{proposal, old}, 1, "requirements", nil); !ok || got.Version != 2 {
		t.Fatal("could not select an approved replan")
	}
	if _, ok := ExecutableVersion([]VersionEvaluation{proposal, old}, 2, "requirements", nil); ok {
		t.Fatal("selected the previous replan after another back transition")
	}
	operation := ExternalOperation{"publish", "repo", hash, hash, hash}
	p.ExternalOperations = []ExternalOperation{operation}
	if !p.RequiresApproval("requirements", nil) || !p.Declares(operation) {
		t.Fatal("could not match an external operation declaration")
	}
	changed := operation
	changed.ExecutableHash = strings.Repeat("0", 64)
	if p.Declares(changed) {
		t.Fatal("accepted a declaration with a changed executable")
	}
	approved := VersionEvaluation{Plan: p, Judgment: PlanPass}
	if approved.Executable("requirements", nil) {
		t.Fatal("made an unapproved external operation executable")
	}
	approved.Approved = true
	if !approved.Executable("requirements", nil) {
		t.Fatal("could not make a passed and approved plan executable")
	}
}

func TestStageLimits(t *testing.T) {
	limits := Limits{MaxRounds: 3, MaxNoProgress: 2}
	if limits.Reached(2, 1) || !limits.Reached(3, 0) || !limits.Reached(1, 2) {
		t.Fatal("incorrect stage limits")
	}
}

func TestTransitions(t *testing.T) {
	transitions := Transitions{Fix: "a", Ask: "b", Back: "c"}
	for action, want := range map[Transition]*string{Fix: &transitions.Fix, Ask: &transitions.Ask, Back: &transitions.Back, Done: nil} {
		got, err := transitions.Next(action)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("transition %s: %v, %v", action, got, err)
		}
	}
	if _, err := transitions.Next("unknown"); err == nil {
		t.Fatal("accepted an unknown action")
	}
}

func TestContentHash(t *testing.T) {
	if ContentHash([]byte("{}")) == ContentHash([]byte("{}\n")) {
		t.Fatal("did not hash the original bytes")
	}
}

func TestReplanTransition(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	if template.Stages[0].Transitions.Done == nil || *template.Stages[0].Transitions.Done != "replan" {
		t.Fatal("default purpose does not replan")
	}
	p := newTestPlan(template, hash, "Define requirements", []CompletionCondition{{ID: "C1", Description: "Requirements are agreed"}})
	bindChecks(&p)
	if err := v.ValidatePlan(p); err != nil {
		t.Fatal(err)
	}
	for _, action := range []Transition{Fix, Ask, Back} {
		bad := clonePlan(p)
		target := "replan"
		switch action {
		case Fix:
			bad.Stages[0].Transitions.Fix = target
		case Ask:
			bad.Stages[0].Transitions.Ask = target
		case Back:
			bad.Stages[0].Transitions.Back = target
		}
		if err := v.ValidatePlan(bad); err == nil {
			t.Fatalf("accepted replan for %s", action)
		}
	}
	p.Stages[0].ID = "replan"
	if err := v.ValidatePlan(p); err == nil {
		t.Fatal("accepted reserved stage name")
	}
}

func newTestPlan(pattern PlanTemplate, hash, task string, conditions []CompletionCondition) Plan {
	p := Plan{Version: 1, Pattern: PatternRef{pattern.Name, hash}, Task: task, CompletionConditions: conditions, StartStage: pattern.StartStage, Stages: pattern.Stages, ExternalOperations: []ExternalOperation{}}
	for i, stage := range p.Stages {
		if stage.Transitions.Done != nil && *stage.Transitions.Done == "replan" {
			p.Stages = p.Stages[:i+1]
			break
		}
	}
	return clonePlan(p)
}

func TestBundledPatterns(t *testing.T) {
	v := validator(t)
	for _, name := range []string{"default", "fix", "docs"} {
		pattern, hash := loadTemplate(t, v, name)
		p := newTestPlan(pattern, hash, "Complete the task", []CompletionCondition{{"C1", "Expected behavior"}})
		bindChecks(&p)
		if err := v.ValidatePlan(p); err != nil {
			t.Fatal(err)
		}
		if p.Pattern.Name != name {
			t.Fatal("pattern reference was lost")
		}
	}
}
