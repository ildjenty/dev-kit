package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// withConditions binds every check of stage to the given completion condition IDs.
func withConditions(stage plan.Stage, ids ...string) plan.Stage {
	for i := range stage.Checks {
		stage.Checks[i].CompletionConditionIDs = slices.Clone(ids)
	}
	return stage
}

// currentStageJSON returns the current stage block of a work brief.
func currentStageJSON(t *testing.T, body string) plan.Stage {
	t.Helper()
	_, rest, found := strings.Cut(body, "## Current stage\n\n```json\n")
	if !found {
		t.Fatalf("work brief lacks the current stage:\n%s", body)
	}
	block, _, _ := strings.Cut(rest, "\n```")
	var stage plan.Stage
	if err := json.Unmarshal([]byte(block), &stage); err != nil {
		t.Fatal(err)
	}
	return stage
}

func latestStageID(t *testing.T, e *Engine) string {
	t.Helper()
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	return run.StageID
}

func TestIncrementConditionIDsReachEachBrief(t *testing.T) {
	e, start := startRules(t, fixture)
	expected := map[string][]string{"implement-1": {"C1"}, "verification-1": {"C1"}, "implement-2": {"C2"}, "verification-2": {"C2"}, "acceptance": {"C1", "C2"}, "retrospective": {"C1", "C2"}}
	p := composedPlan(t, e, 1,
		withConditions(implementInstance("implement-1", doneTo("verification-1")), "C1"),
		withConditions(verificationInstance("verification-1", doneTo("implement-2")), "C1"),
		withConditions(implementInstance("implement-2", doneTo("verification-2")), "C2"),
		withConditions(verificationInstance("verification-2", doneTo("acceptance")), "C2"),
		withConditions(verificationInstance("acceptance", doneTo("retrospective")), "C1", "C2"),
		withConditions(retrospectiveInstance("retrospective"), "C1", "C2"))
	p.CompletionConditions = []plan.CompletionCondition{{ID: "C1", Description: "The first increment works end to end"}, {ID: "C2", Description: "The second increment works end to end"}}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	workers, judges := map[string]bool{}, map[string]bool{}
	inspect := func(instruction *form.Instruction) {
		switch instruction.Kind {
		case form.InstructionWork:
			stageID := latestStageID(t, e)
			for _, item := range currentStageJSON(t, briefText(t, instruction)).Checks {
				if !slices.Equal(item.CompletionConditionIDs, expected[stageID]) {
					t.Fatalf("%s worker brief carries %v for %s, want %v", stageID, item.CompletionConditionIDs, item.Aspect, expected[stageID])
				}
			}
			workers[stageID] = true
		case form.InstructionJudgment:
			stageID := latestStageID(t, e)
			data, err := e.readInstruction(instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(data.Request.CompletionConditionIDs, expected[stageID]) {
				t.Fatalf("%s judge of %s received %v, want %v", stageID, data.Request.Aspect.ID, data.Request.CompletionConditionIDs, expected[stageID])
			}
			ids, err := json.MarshalIndent(expected[stageID], "  ", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(briefText(t, instruction), `"completion_condition_ids": `+string(ids)) {
				t.Fatalf("%s judge brief of %s omits its condition IDs", stageID, data.Request.Aspect.ID)
			}
			judges[stageID] = true
		}
	}
	for range 200 {
		result, err := e.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		instructions := result.Instructions
		if len(instructions) == 0 && result.Instruction != nil {
			instructions = []form.Instruction{*result.Instruction}
		}
		for i := range instructions {
			inspect(&instructions[i])
			if instructions[i].Kind == form.InstructionPlan {
				t.Fatalf("the single plan replanned: %+v", instructions[i])
			}
			if driveInstruction(t, e, &instructions[i]) {
				for stageID := range expected {
					if !workers[stageID] || !judges[stageID] {
						t.Fatalf("%s did not issue both a worker and a judge brief: %v %v", stageID, workers, judges)
					}
				}
				return
			}
		}
	}
	t.Fatal("the plan did not complete")
}

func TestRetrospectiveAuthorityEditReachesNextWork(t *testing.T) {
	e, ctx := fixture(t)
	const marker = "Renaming private helpers is delegated (retrospective marker 7c1)."
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Review the loop"})
	if err != nil {
		t.Fatal(err)
	}
	planResultWith(t, e, start, func(p *plan.Plan) {
		p.Stages = []plan.Stage{retrospectiveInstance("retrospective")}
		p.Stages[0].Checks = []plan.Check{}
		p.StartStage = "retrospective"
	})
	retrospective := checkedNext(t, e)
	if retrospective.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected retrospective work: %+v", retrospective)
	}
	authority, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/authority.md"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), ".devkit/authority.md", string(authority)+"\n- "+marker+"\n")
	submitWork(t, e, retrospective)
	approval := checkedResult(t, e)
	if approval.Instruction.Question == nil || approval.Instruction.Question.StopCategory != "meta_loop_edit" {
		t.Fatalf("the authority edit was not confirmed as a loop edit: %+v", approval)
	}
	answerResult(t, e, approval, ledger.AnswerApproved)
	if done := checkedResult(t, e); done.Status != ledger.WorkCompleted {
		t.Fatalf("the retrospective did not complete: %+v", done)
	}
	if committed := git(t, e.Root(), "show", "HEAD:.devkit/authority.md"); !strings.Contains(committed, marker) {
		t.Fatal("the approved authority edit was not committed")
	}

	next, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	once := func(role string, instruction *form.Instruction) {
		t.Helper()
		if count := strings.Count(briefText(t, instruction), marker); count != 1 {
			t.Fatalf("%s brief carries the updated authority %d times", role, count)
		}
	}
	once("planner", next.Instruction)
	planResultWith(t, e, next, func(p *plan.Plan) {
		p.Stages[0].Checks = []plan.Check{{Aspect: "diff", Required: true, Target: "Documentation", CompletionConditionIDs: []string{"C1"}}}
	})
	seen := map[form.InstructionKind]bool{}
	for range 40 {
		result, err := e.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == ledger.WorkCompleted {
			break
		}
		instruction := result.Instruction
		if instruction == nil {
			t.Fatalf("missing instruction: %+v", result)
		}
		switch instruction.Kind {
		case form.InstructionPlanCheck, form.InstructionJudgment:
			once(string(instruction.Kind), instruction)
			data, err := e.readInstruction(instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			collect(t, e, instruction, pass(*data.Request))
		case form.InstructionWork:
			once("worker", instruction)
			write(t, e.Root(), "README.md", "# Updated documentation\n")
			submitWork(t, e, result)
		case form.InstructionQuestion:
			answerResult(t, e, result, ledger.AnswerApproved)
		default:
			t.Fatalf("unexpected instruction: %+v", instruction)
		}
		seen[instruction.Kind] = true
	}
	for _, kind := range []form.InstructionKind{form.InstructionPlanCheck, form.InstructionWork, form.InstructionJudgment} {
		if !seen[kind] {
			t.Fatalf("the second work issued no %s brief", kind)
		}
	}
}
