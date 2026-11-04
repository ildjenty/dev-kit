package plan

import (
	"encoding/json"
	"io/fs"
	"os"
	"reflect"
	"slices"
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

func TestDefaultsAndExamples(t *testing.T) {
	v := validator(t)
	for _, name := range []string{"default", "feature", "fix", "docs"} {
		t.Run(name, func(t *testing.T) {
			template, hash := loadTemplate(t, v, name)
			p := NewPlan(template, hash, "Fix wording", []CompletionCondition{{"C1", "Wording is correct"}})
			bindChecks(&p)
			if err := v.ValidatePlan(p); err != nil {
				t.Fatal(err)
			}
			if got := Deviations(template, p); len(got) != 0 {
				t.Fatalf("original plan was classified as a deviation: %+v", got)
			}
		})
	}
	data, err := os.ReadFile("../../docs/architecture/examples.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := strings.Split(string(data), "```json\n")
	if _, err := v.ParseTemplate([]byte(strings.Split(blocks[1], "```")[0])); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ParsePlan([]byte(strings.Split(blocks[2], "```")[0])); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaAndReferences(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "Fix wording", []CompletionCondition{{"C1", "Wording is correct"}})
	if err := v.ValidatePlan(p); err == nil {
		t.Fatal("accepted a plan without check targets and completion condition mappings")
	}
	bindChecks(&p)
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Stages[0].Checks[0].CompletionConditionIDs = []string{"missing"} },
		func(p *Plan) { p.Stages[0].Transitions.Back = "missing" },
		func(p *Plan) { p.Stages[0].Checks[0].Aspect = "different" },
		func(p *Plan) { p.Stages = append(p.Stages, p.Stages[0]) },
		func(p *Plan) { p.Stages[0].Limits.MaxRounds = 0 },
		func(p *Plan) { p.Stages[0].Assignee = "judge" },
	} {
		bad := clonePlan(p)
		change(&bad)
		if err := v.ValidatePlan(bad); err == nil {
			t.Fatal("accepted an invalid plan")
		}
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

func TestSelectionAndDeviations(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	p := NewPlan(template, hash, "Fix internal behavior", []CompletionCondition{{"C1", "Works as expected"}})
	selected, err := p.SelectStages(template, []string{"research", "contract", "implement"})
	if err != nil {
		t.Fatal(err)
	}
	bindChecks(&selected)
	if err := v.ValidatePlan(selected); err != nil {
		t.Fatal(err)
	}
	wantSelection := []Deviation{
		{Kind: ChangedTransition, StageID: "research", Transition: Done},
		{Kind: ChangedTransition, StageID: "contract", Transition: Back},
		{Kind: ChangedTransition, StageID: "contract", Transition: Done},
		{Kind: ChangedTransition, StageID: "implement", Transition: Back},
	}
	if got := Deviations(template, selected); !reflect.DeepEqual(got, wantSelection) {
		t.Fatalf("stage selection deviations: got %+v, want %+v", got, wantSelection)
	}
	if selected.StartStage != "research" || *selected.Stages[0].Transitions.Done != "contract" {
		t.Fatal("optional stage insertion and start stage change were not applied")
	}
	if template.Stages[3].Transitions.Back != "structure" || p.Stages[0].Checks[0].Target != "" {
		t.Fatal("modified the original template or plan")
	}
	leading, err := p.SelectStages(template, []string{"contract", "steps", "implement"})
	if err != nil {
		t.Fatal(err)
	}
	wantLeading := []Deviation{{Kind: ChangedTransition, StageID: "contract", Transition: Back}}
	if got := Deviations(template, leading); !reflect.DeepEqual(got, wantLeading) {
		t.Fatalf("omitting leading stages should only change transitions: %+v", got)
	}
	leading.StartStage = "implement"
	wantLeading = append([]Deviation{{Kind: ChangedStartStage, StageID: "implement"}}, wantLeading...)
	if got := Deviations(template, leading); !reflect.DeepEqual(got, wantLeading) {
		t.Fatalf("missed a change to a retained start stage: %+v", got)
	}
	p.Stages[1].Gate.Required = false
	p.Stages[1].Transitions.Back = "implement"
	p.Stages[1].Checks = nil
	got := Deviations(template, p)
	want := []DeviationKind{RemovedGate, ChangedTransition, RemovedRequiredCheck}
	if len(got) != len(want) {
		t.Fatalf("deviations: %+v", got)
	}
	for i, kind := range want {
		if got[i].Kind != kind {
			t.Fatalf("deviations: %+v", got)
		}
	}
	unknown := cloneStage(p.Stages[0])
	unknown.ID = "custom"
	p.Stages = append(p.Stages, unknown)
	if got := Deviations(template, p); got[len(got)-1].Kind != AddedStage {
		t.Fatalf("addition of a stage absent from optional stages: %+v", got)
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
					stage.Checks = []Check{{Aspect: "wording", Runner: "review", Required: true}}
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

func TestTemplateStartStageDeviations(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	template.StartStage = "scope"
	p := NewPlan(template, hash, "Fix behavior", []CompletionCondition{{"C1", "Works as expected"}})
	if got := Deviations(template, p); len(got) != 0 {
		t.Fatalf("template start stage was classified as a deviation: %+v", got)
	}
	p.StartStage = "purpose"
	if got := Deviations(template, p); len(got) != 1 || got[0].Kind != ChangedStartStage {
		t.Fatalf("missed a change from the template start stage: %+v", got)
	}
}

func TestOptionalStageTransitionRedirection(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	template.OptionalStages[0].Transitions.Fix = "steps"
	template.OptionalStages[0].Transitions.Ask = "structure"
	template.OptionalStages[0].Transitions.Back = "steps"
	p := NewPlan(template, hash, "Fix behavior", []CompletionCondition{{"C1", "Works as expected"}})
	selected, err := p.SelectStages(template, []string{"research", "contract", "implement"})
	if err != nil {
		t.Fatal(err)
	}
	bindChecks(&selected)
	if err := v.ValidatePlan(selected); err != nil {
		t.Fatal(err)
	}
	stage, _ := selected.Stage("research")
	if stage.Transitions.Fix != "research" || stage.Transitions.Ask != "research" || stage.Transitions.Back != "contract" {
		t.Fatalf("optional stage transitions were not redirected: %+v", stage.Transitions)
	}
	got := Deviations(template, selected)
	if len(got) != 7 {
		t.Fatalf("redirected stage transition deviations: %+v", got)
	}
	for _, action := range []Transition{Fix, Ask, Back, Done} {
		if !slices.Contains(got, Deviation{Kind: ChangedTransition, StageID: "research", Transition: action}) {
			t.Fatalf("missed a redirected optional stage transition: %s", action)
		}
	}
}

func TestStageSelectionRequiresTransitionApproval(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	p := NewPlan(template, hash, "Fix behavior", []CompletionCondition{{"C1", "Works as expected"}})
	selected, err := p.SelectStages(template, []string{"purpose", "implement"})
	if err != nil {
		t.Fatal(err)
	}
	bindChecks(&selected)
	if err := v.ValidatePlan(selected); err != nil {
		t.Fatal(err)
	}
	want := []Deviation{
		{Kind: ChangedTransition, StageID: "purpose", Transition: Done},
		{Kind: ChangedTransition, StageID: "implement", Transition: Back},
	}
	if got := Deviations(template, selected); !reflect.DeepEqual(got, want) {
		t.Fatalf("stage selection deviations: got %+v, want %+v", got, want)
	}
	evaluation := VersionEvaluation{Plan: selected, Judgment: PlanPass, Origin: InitialProposal}
	if evaluation.Executable(template) {
		t.Fatal("made redirected transitions executable without approval")
	}
	evaluation.Approved = true
	if !evaluation.Executable(template) {
		t.Fatal("could not execute approved redirected transitions")
	}
}

func TestDeviationsWithUnselectedOptionalTransition(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	template.OptionalStages[0].Transitions.Back = "prototype"
	template.OptionalStages[0].Gate.Required = true
	template.OptionalStages[0].Checks = []Check{{Aspect: "research", Runner: "review", Required: true}}
	if err := v.ValidateTemplate(template); err != nil {
		t.Fatal(err)
	}
	p := NewPlan(template, hash, "Fix behavior", []CompletionCondition{{"C1", "Works as expected"}})
	research := cloneStage(template.OptionalStages[0])
	research.Transitions.Back = "research"
	research.Gate.Required = false
	research.Checks = []Check{}
	p.Stages = append(p.Stages, research)
	bindChecks(&p)
	if err := v.ValidatePlan(p); err != nil {
		t.Fatal(err)
	}
	want := []Deviation{
		{Kind: RemovedGate, StageID: "research"},
		{Kind: ChangedTransition, StageID: "research", Transition: Back},
		{Kind: RemovedRequiredCheck, StageID: "research", Aspect: "research", Runner: "review"},
	}
	if got := Deviations(template, p); !reflect.DeepEqual(got, want) {
		t.Fatalf("optional stage deviations: got %+v, want %+v", got, want)
	}
}

func TestCheckMappingsWithSharedAspectAndRunner(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "Fix behavior", []CompletionCondition{
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
		{"different runner", func(c *Check) {
			c.Runner = "other-review"
			c.Target = "another target"
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := clonePlan(p)
			duplicate := cloneCheck(candidate.Stages[0].Checks[1])
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

func TestSelectionRejectsUnresolvedTransitions(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	template.OptionalStages[0].Transitions.Back = "prototype"
	p := NewPlan(template, hash, "Fix behavior", nil)
	if _, err := p.SelectStages(template, []string{"research", "implement"}); err == nil {
		t.Fatal("accepted a reference to an unselected optional stage")
	}
}

func TestWithdrawnHumanProposalAfterReplanning(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "Fix behavior", []CompletionCondition{{"C1", "Works as expected"}})
	bindChecks(&p)
	replan, _ := Replan(p, p)
	proposal, _ := Replan(replan, replan)
	versions := []VersionEvaluation{
		{Plan: p, Judgment: PlanPass, Origin: InitialProposal},
		{Plan: replan, Judgment: PlanPass, Origin: BackProposal, Approved: true},
		{Plan: proposal, Origin: HumanProposal, Withdrawn: true},
	}
	if got, ok := ExecutableVersion(template, versions, 1); !ok || got.Version != 2 {
		t.Fatal("could not select an approved replan after a voluntary human withdrawal")
	}
	if _, ok := ExecutableVersion(template, versions, 2); ok {
		t.Fatal("selected an old version after a voluntary human withdrawal")
	}
}

func TestVersionsAndExternalApproval(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "Fix", []CompletionCondition{{"C1", "Correct"}})
	bindChecks(&p)
	next, diff := Replan(p, p)
	if next.Version != 2 || len(diff.ChangedFields) != 0 || len(diff.ChangedStages) != 0 {
		t.Fatalf("replanning: %+v, %+v", next, diff)
	}
	next.Stages[0].Checks[0].CompletionConditionIDs[0] = "changed"
	if p.Stages[0].Checks[0].CompletionConditionIDs[0] != "C1" {
		t.Fatal("modified the old plan")
	}
	old := VersionEvaluation{Plan: p, Judgment: PlanPass, Origin: InitialProposal}
	proposal := VersionEvaluation{Plan: next, Origin: HumanProposal}
	if _, ok := ExecutableVersion(template, []VersionEvaluation{old, proposal}, 0); ok {
		t.Fatal("selected an old plan while a proposal was pending")
	}
	proposal.Withdrawn = true
	if got, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 0); !ok || got.Version != 1 {
		t.Fatal("could not return to the old plan after a voluntary human withdrawal")
	}
	proposal.Origin = BackProposal
	if _, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 1); ok {
		t.Fatal("selected an old plan after a replanning withdrawal")
	}
	proposal.Withdrawn = false
	proposal.Judgment = PlanPass
	if proposal.Executable(template) {
		t.Fatal("made a back-triggered replan executable without human approval")
	}
	proposal.Approved = true
	if got, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 1); !ok || got.Version != 2 {
		t.Fatal("could not select an approved replan")
	}
	if _, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 2); ok {
		t.Fatal("selected the previous replan after another back transition")
	}
	operation := ExternalOperation{"publish", "repo", hash, hash, hash}
	p.ExternalOperations = []ExternalOperation{operation}
	if !p.RequiresApproval(template) || !p.Declares(operation) {
		t.Fatal("could not match an external operation declaration")
	}
	changed := operation
	changed.ExecutableHash = strings.Repeat("0", 64)
	if p.Declares(changed) {
		t.Fatal("accepted a declaration with a changed executable")
	}
	approved := VersionEvaluation{Plan: p, Judgment: PlanPass}
	if approved.Executable(template) {
		t.Fatal("made an unapproved external operation executable")
	}
	approved.Approved = true
	if !approved.Executable(template) {
		t.Fatal("could not make a passed and approved plan executable")
	}
}

func TestStageLimits(t *testing.T) {
	limits := Limits{MaxRounds: 3, MaxNoProgress: 2}
	if limits.Reached(2, 1) || !limits.Reached(3, 0) || !limits.Reached(1, 2) {
		t.Fatal("incorrect stage limits")
	}
}

func TestPlanLimits(t *testing.T) {
	template := PlanTemplate{}
	if template.PlanRoundsReached(2) || !template.PlanRoundsReached(3) {
		t.Fatal("incorrect default plan limit")
	}
	template.PlanLimits = &PlanLimits{MaxRounds: 5}
	if template.PlanRoundsReached(3) || !template.PlanRoundsReached(5) {
		t.Fatal("plan limit override was not applied")
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
