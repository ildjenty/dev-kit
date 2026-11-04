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
				write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n[approval]\nplan = '"+policy+"'\n"))
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
				started, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
				if err != nil {
					t.Fatal(err)
				}
				// A plan ends its done path at replan; the replanned plan holds the later stages.
				initial := planResultWith(t, e, started, func(p *plan.Plan) {
					if mode == "replan" {
						p.Stages = p.Stages[:1]
					}
				})
				raw := func() Result {
					t.Helper()
					result, err := e.Next(ctx)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				passPlan := func() {
					t.Helper()
					review := raw()
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
				work := raw()
				{
					if work.Instruction.Question == nil {
						t.Fatal("the first plan was not approved")
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
					if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: replanning.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Confirm how to continue."), Obstacles: []string{"Rebuild remaining work"}})); err != nil {
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
				remaining.Stages = template.Stages[1:]
				collect(t, e, replanning.Instruction, form.PlannerResult{InstructionID: replanning.Instruction.ID, Plan: marshal(t, remaining)})
				passPlan()
				next := raw()
				// After replan, the stages beyond the approved plan are a structural difference.
				if mode == "replan" && policy == "deviation" {
					if next.Instruction.Question == nil || next.Instruction.Question.StopCategory != "plan_approval" {
						t.Fatalf("new stages after replan bypassed approval: %+v", next)
					}
					next = approvedNext(t, e, next)
				}
				if next.Instruction == nil || next.Instruction.Kind != form.InstructionWork {
					t.Fatalf("completed stage triggered repeated plan approval: %+v", next)
				}
				state, err := e.load()
				if err != nil {
					t.Fatal(err)
				}
				text, err := e.planApprovalText(ctx, state, remaining, "HEAD")
				if err != nil || strings.Contains(text, `"StageID": "purpose"`) {
					t.Fatalf("approval question lists the completed stage as removed: %s: %v", text, err)
				}
				remaining.Stages = remaining.Stages[:1]
				remaining.Stages[0].Transitions.Done = nil
				text, err = e.planApprovalText(ctx, state, remaining, "HEAD")
				// Only an approved stage left out of the proposal is a difference; after replan
				// the retrospective was never approved.
				if err != nil || strings.Contains(text, `"StageID": "retrospective"`) != (mode == "back") {
					t.Fatalf("an uncompleted omitted stage lost its deviation: %s: %v", text, err)
				}
			})
		}
	}
}
