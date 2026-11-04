package plan

import (
	"reflect"
	"slices"
	"testing"
)

func pathStage(id, definition, done string) Stage {
	stage := Stage{ID: id, Definition: definition, Assignee: Worker, Limits: Limits{3, 2}, Checks: []Check{}, Transitions: Transitions{Fix: id, Ask: id, Back: id}}
	if done != "" {
		stage.Transitions.Done = &done
	}
	return stage
}

func TestPathRulesAcrossReplanning(t *testing.T) {
	rules := []PathRule{{Definition: "implement", Follows: "verification"}, {Require: []string{"purpose"}}, {Last: "retrospective"}, {First: "purpose"}}
	purpose := pathStage("purpose-1", "purpose", "replan")
	initial := Plan{StartStage: purpose.ID, Stages: []Stage{purpose}}
	if got := EvaluatePath(nil, initial, rules); len(got) != 0 {
		t.Fatalf("initial segment: %+v", got)
	}
	stages := []Stage{pathStage("code-1", "implement", "verify-1"), pathStage("verify-1", "verification", "retro-1"), pathStage("retro-1", "retrospective", "")}
	p := Plan{StartStage: "code-1", Stages: stages}
	before := clonePlan(p)
	if got := EvaluatePath([]Stage{purpose}, p, rules); len(got) != 0 {
		t.Fatalf("adopted path: %+v", got)
	}
	if !reflect.DeepEqual(before, p) {
		t.Fatal("path validation mutated the plan")
	}
	p.Stages[1].Definition = "edit"
	if got := EvaluatePath([]Stage{purpose}, p, rules); len(got) != 1 || got[0].StageID != "code-1" {
		t.Fatalf("missing verification: %+v", got)
	}
	p = clonePlan(before)
	p.Stages[0].Transitions.Done = doneTarget("replan")
	p.Stages = p.Stages[:1]
	if got := EvaluatePath([]Stage{purpose}, p, rules); len(got) != 1 || got[0].Rule != 0 {
		t.Fatalf("follows crossed replan: %+v", got)
	}
	completed := []Stage{purpose, pathStage("earlier-code", "implement", "replan")}
	p = clonePlan(before)
	if got := EvaluatePath(completed, p, rules); len(got) != 1 || got[0].StageID != "earlier-code" {
		t.Fatalf("completed segment not checked: %+v", got)
	}
}

func TestPathRuleFailuresAndRepeatedDefinitions(t *testing.T) {
	stages := []Stage{pathStage("a", "implement", "b"), pathStage("b", "verification", "c"), pathStage("c", "implement", "d"), pathStage("d", "verification", "")}
	p := Plan{StartStage: "a", Stages: stages}
	if got := EvaluatePath(nil, p, []PathRule{{Definition: "implement", Follows: "verification"}}); len(got) != 0 {
		t.Fatal(got)
	}
	rules := []PathRule{{Require: []string{"scope"}}, {First: "scope"}, {Last: "retrospective"}}
	if got := EvaluatePath(nil, p, rules); len(got) != 3 {
		t.Fatalf("missing final path violations: %+v", got)
	}
	p.Stages[3].Transitions.Done = doneTarget("missing")
	if len(EvaluatePath(nil, p, nil)) == 0 {
		t.Fatal("accepted missing done target")
	}
	p.Stages[3].Transitions.Done = doneTarget("a")
	if len(EvaluatePath(nil, p, nil)) == 0 {
		t.Fatal("accepted cyclic path")
	}
}

func TestStageRules(t *testing.T) {
	stage := pathStage("code-1", "implement", "")
	stage.Checks = []Check{{Aspect: "test", Required: true}}
	outputs := []string{}
	rule := StageRule{Definition: []string{"implement"}, Checks: []string{"test"}, Roles: []string{"worker"}, MaxRounds: 3, MaxNoProgress: 2}
	p := Plan{Stages: []Stage{stage}}
	if got := ValidateStageRules(p, []StageRule{rule}, Limits{3, 2}); len(got) != 0 {
		t.Fatal(got)
	}
	for _, change := range []func(*StageRule){
		func(r *StageRule) { r.Checks = []string{"diff"} },
		func(r *StageRule) { r.Gate = true },
		func(r *StageRule) { r.Roles = []string{"human"} },
		func(r *StageRule) { r.MaxRounds = 2 },
		func(r *StageRule) { r.MaxNoProgress = 1 },
		func(r *StageRule) { r.Report = &Selection{"templates", "report"} },
		func(r *StageRule) { r.History = HistoryAll },
		func(r *StageRule) { r.Outputs = &outputs },
	} {
		candidate := rule
		change(&candidate)
		if got := ValidateStageRules(p, []StageRule{candidate}, Limits{3, 2}); len(got) != 1 {
			t.Fatalf("missing stage violation: %+v", got)
		}
	}
	p.Stages[0].Limits.MaxRounds = 4
	if got := ValidateStageRules(p, nil, Limits{3, 2}); len(got) != 1 {
		t.Fatal("missing global ceiling violation")
	}
	rule.Definition = []string{"*"}
	p.Stages[0].Assignee = "other"
	if got := ValidateStageRules(p, []StageRule{rule}, Limits{4, 2}); len(got) != 2 {
		t.Fatalf("wildcard did not apply: %+v", got)
	}
}

func TestInstanceValidation(t *testing.T) {
	stage := pathStage("edit-1", "edit", "")
	p := Plan{StartStage: stage.ID, Stages: []Stage{stage}}
	p.Stages[0].Transitions.Back = "old-instance"
	if err := ValidateBackTargets(p, nil); err == nil {
		t.Fatal("accepted unknown back target")
	}
	if err := ValidateBackTargets(p, map[string]bool{"old-instance": true}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReturnTarget(p, "old-instance"); err == nil {
		t.Fatal("accepted plan missing return instance")
	}
	if err := ValidateReturnTarget(p, "edit-1"); err != nil {
		t.Fatal(err)
	}
	if err := validateReachable(p.StartStage, append(p.Stages, pathStage("edit-2", "edit", ""))); err == nil {
		t.Fatal("accepted unreachable instance")
	}
	p.Stages[0].Transitions.Ask = "old-instance"
	if err := validateStageTransitions(p.Stages[0], p.Stages, map[string]bool{"edit-1": true}, true); err == nil {
		t.Fatal("accepted ask targeting a different instance")
	}
}

func TestStructuralDifference(t *testing.T) {
	before := Plan{StartStage: "edit-1", Stages: []Stage{pathStage("edit-1", "edit", "")}}
	after := clonePlan(before)
	after.Pattern = PatternRef{Name: "other"}
	after.Task = "Changed task"
	if got := StructuralDifference(before, after); len(got) != 0 {
		t.Fatalf("nonstructural changes included: %+v", got)
	}
	after.Stages[0].Gate.Required = true
	after.Stages[0].Instructions = []string{"Review links"}
	after.Stages[0].Definition = "implement"
	got := StructuralDifference(before, after)
	for _, field := range []string{"gate", "instructions", "definition"} {
		if !slices.ContainsFunc(got, func(c StructuralChange) bool { return c.Field == field && c.StageID == "edit-1" }) {
			t.Fatalf("missing field difference %s: %+v", field, got)
		}
	}
	if len(StructuralDifference(after, before)) != len(got) {
		t.Fatal("structural comparison is not symmetric")
	}
	after.Stages = append(after.Stages, pathStage("verify", "verification", ""))
	after.StartStage = "verify"
	diff := Difference(before, after)
	if !slices.Contains(diff.AddedStages, "verify") || !slices.Contains(diff.ChangedFields, "pattern") {
		t.Fatal(diff)
	}
}

func TestRemainingApprovalBaseline(t *testing.T) {
	p := Plan{Version: 1, StartStage: "edit-1", Stages: []Stage{pathStage("edit-1", "edit", "retro-1"), pathStage("retro-1", "retrospective", "")}}
	p.CompletionConditions = []CompletionCondition{{"C1", "Correct text"}}
	completed := map[string]bool{"edit-1": true}
	remaining := RemainingPlan(p, completed)
	if remaining.StartStage != "retro-1" || len(remaining.Stages) != 1 || len(p.Stages) != 2 {
		t.Fatal("incorrect remaining approval baseline")
	}
	candidate := clonePlan(remaining)
	candidate.Version = 2
	versions := []VersionEvaluation{{Plan: p, Approved: true, Judgment: PlanPass}, {Plan: candidate, Judgment: PlanPass}}
	if _, ok := ExecutableVersion(versions, 0, "deviation", completed); !ok {
		t.Fatal("completed instances caused a structural approval")
	}
	candidate.Stages[0].Instructions = []string{"Review recurrence prevention"}
	versions[1].Plan = candidate
	if _, ok := ExecutableVersion(versions, 0, "deviation", completed); ok {
		t.Fatal("structural change bypassed approval")
	}
	completed["retro-1"] = true
	if got := RemainingPlan(p, completed); len(got.Stages) != 0 || got.StartStage != "" {
		t.Fatal("incorrect empty remainder")
	}
}
