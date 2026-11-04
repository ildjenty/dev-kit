package engine

import (
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestCompletedStagesAreNotReplanDeviations(t *testing.T) {
	for _, mode := range []string{"replan", "back"} {
		for _, policy := range []string{"requirements", "deviation"} {
			t.Run(mode+"/"+policy, func(t *testing.T) {
				e, ctx := fixture(t)
				write(t, e.Root(), "devkit.toml", "kit_version = 'v3'\n[approval]\nplan = '"+policy+"'\n[detection]\nrequired = false\n")
				template := plan.PlanTemplate{Name: "test", StartStage: "purpose", OptionalStages: []plan.Stage{}}
				ids := []string{"purpose", "implement", "retrospective"}
				for index, id := range ids {
					stage := plan.Stage{ID: id, Assignee: plan.Worker, Checks: []plan.Check{}, Gate: plan.Gate{Reason: "Test stage"}, Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2}, Transitions: plan.Transitions{Fix: id, Ask: id, Back: id}}
					if index+1 < len(ids) {
						next := ids[index+1]
						stage.Transitions.Done = &next
					}
					template.Stages = append(template.Stages, stage)
				}
				if mode == "replan" {
					next := "replan"
					template.Stages[0].Transitions.Done = &next
				}
				write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
				git(t, e.Root(), "add", ".")
				git(t, e.Root(), "commit", "-qm", "Configure remaining-plan test")
				started, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
				if err != nil {
					t.Fatal(err)
				}
				initial := planResult(t, e, started)
				passPlan := func() {
					t.Helper()
					review := nextResult(t, e, ctx)
					if review.Instruction == nil || review.Instruction.Kind != form.InstructionPlanCheck {
						t.Fatalf("expected a plan check: %+v", review)
					}
					data, err := e.readInstruction(review.Instruction.ID)
					if err != nil {
						t.Fatal(err)
					}
					collect(t, e, review.Instruction, pass(*data.Request))
				}
				passPlan()
				work := nextResult(t, e, ctx)
				if policy == "requirements" {
					if work.Instruction.Question == nil {
						t.Fatal("first requirements plan was not approved")
					}
					observeAnswer(t, e, "Approve these completion conditions.")
					if _, err := e.Answer(ctx, AnswerRequest{QuestionID: work.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
						t.Fatal(err)
					}
					work = nextResult(t, e, ctx)
				}
				if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
					t.Fatal(err)
				}
				replanning := nextResult(t, e, ctx)
				if mode == "back" {
					if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: replanning.Instruction.ID, TerminationKind: form.TerminationQuestion, Obstacles: []string{"Rebuild remaining work"}})); err != nil {
						t.Fatal(err)
					}
					question := nextResult(t, e, ctx)
					observeAnswer(t, e, "Return and rebuild the remaining plan with the same conditions.")
					if _, err := e.Answer(ctx, AnswerRequest{QuestionID: question.Instruction.Question.ID, Interpretation: ledger.AnswerBack}); err != nil {
						t.Fatal(err)
					}
					replanning = nextResult(t, e, ctx)
				}
				if replanning.Instruction == nil || replanning.Instruction.Kind != form.InstructionPlan {
					t.Fatalf("expected remaining-plan instruction: %+v", replanning)
				}
				remaining := initial
				remaining.Version, remaining.StartStage = 2, "implement"
				remaining.Stages = initial.Stages[1:]
				collect(t, e, replanning.Instruction, form.PlannerResult{InstructionID: replanning.Instruction.ID, Plan: marshal(t, remaining)})
				passPlan()
				next := nextResult(t, e, ctx)
				if next.Instruction == nil || next.Instruction.Kind != form.InstructionWork {
					t.Fatalf("completed stage triggered repeated plan approval: %+v", next)
				}
				state, err := e.load()
				if err != nil {
					t.Fatal(err)
				}
				text, err := e.planApprovalText(state, template, remaining)
				if err != nil || strings.Contains(text, "- purpose (deviation)") || strings.Contains(text, `"removed_stage"`) {
					t.Fatalf("approval question lists the completed stage as removed: %s: %v", text, err)
				}
				remaining.Stages = remaining.Stages[:1]
				remaining.Stages[0].Transitions.Done = nil
				text, err = e.planApprovalText(state, template, remaining)
				if err != nil || !strings.Contains(text, "- retrospective (deviation)") {
					t.Fatalf("an uncompleted omitted stage lost its deviation: %s: %v", text, err)
				}
			})
		}
	}
}
