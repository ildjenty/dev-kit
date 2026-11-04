package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

func briefText(t *testing.T, instruction *form.Instruction) string {
	t.Helper()
	data, err := os.ReadFile(instruction.Brief)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func configure(t *testing.T, e *Engine, settings string) {
	t.Helper()
	write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n[roles.agent]\nkind = 'work'\nagent = 'devkit-worker'\n[approval]\nplan = 'deviation'\n"+settings))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure sources")
}

func submitWork(t *testing.T, e *Engine, work Result) {
	t.Helper()
	if work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected work: %+v", work)
	}
	if _, err := e.Submit(t.Context(), marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Edited the documents"}})); err != nil {
		t.Fatal(err)
	}
}

func TestJudgmentRequestSuppliesSelectedCriteria(t *testing.T) {
	e, ctx := fixture(t)
	configureCatalogEntry(t, e, "wording", "")
	data, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", strings.Replace(string(data), "select = 'review/wording.md' }]", "select = 'review/wording.md' }, { source = 'principles', select = 'designing.md' }]", 1))
	git(t, e.Root(), "commit", "-qam", "Select more criteria")
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	submitWork(t, e, working(t, e, ctx))
	judgment, err := e.Next(ctx)
	if err != nil || judgment.Instruction == nil || judgment.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("expected a judgment: %+v, %v", judgment, err)
	}
	text := briefText(t, judgment.Instruction)
	for _, name := range []string{"principles/review/wording.md", "principles/designing.md"} {
		body, err := os.ReadFile(filepath.Join(e.Root(), ".devkit", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, "Reference: "+name) || !strings.Contains(text, string(body)) {
			t.Fatalf("judgment request omitted selected criteria %s", name)
		}
	}
	unselected, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/principles/review/test.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, string(unselected)) {
		t.Fatal("judgment request included an unselected criteria document")
	}
	issued, err := e.readInstruction(judgment.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(issued.Request.Criteria) != 2 || issued.Request.Criteria[0].ID != "review/wording.md" || issued.Request.Criteria[0].ContentHash == "" {
		t.Fatalf("request did not record the criteria identities: %+v", issued.Request.Criteria)
	}
}

func TestEmptyCriteriaSelectionRejectsThePlan(t *testing.T) {
	e, ctx := fixture(t)
	configureCatalogEntry(t, e, "wording", "")
	data, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", strings.Replace(string(data), "select = 'review/wording.md' }]", "select = 'review/wording.md' }, { source = 'principles', select = 'review/missing-*.md' }]", 1))
	git(t, e.Root(), "commit", "-qam", "Select missing criteria")
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	reissued, err := e.Next(ctx)
	if err != nil || reissued.Instruction == nil || reissued.Instruction.Kind != form.InstructionPlan || reissued.Instruction.ID == start.Instruction.ID {
		t.Fatalf("a plan with an empty criteria selection was not rejected: %+v, %v", reissued, err)
	}
	if body := briefText(t, reissued.Instruction); !strings.Contains(body, "review/missing-*.md") {
		t.Fatalf("the rejection omitted the unresolved selection: %s", body)
	}
}

func TestUnselectedNormDocumentChangeRequiresLoopConfirmation(t *testing.T) {
	e, ctx := fixture(t)
	configure(t, e, "")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	work := checkedNext(t, e)
	write(t, e.Root(), ".devkit/principles/working.md", "# Unselected principle edited by the worker\n")
	submitWork(t, e, work)
	result := checkedNext(t, e)
	if result.Instruction.Question == nil || result.Instruction.Question.StopCategory != "meta_loop_edit" || !strings.Contains(result.Instruction.Question.Text, ".devkit/principles/working.md") {
		t.Fatalf("an unselected norm document change was not confirmed as a loop edit: %+v", result)
	}
}

func outsideSourceWork(t *testing.T) (*Engine, string, string, Result) {
	t.Helper()
	e, ctx := fixture(t)
	products, guides := t.TempDir(), t.TempDir()
	write(t, products, "requirements.md", "Initial outside requirements.\n")
	write(t, guides, "style.md", "Initial outside guide.\n")
	configure(t, e, fmt.Sprintf("[sources.requirements]\npath = %q\n[sources.guides]\npath = %q\nnorm = true\n", products, guides))
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(briefText(t, start.Instruction), "Initial outside requirements.") {
		t.Fatal("planner brief did not read the outside requirements")
	}
	planResult(t, e, start)
	return e, products, guides, checkedNext(t, e)
}

func TestPlanApprovalShowsStructuralDifferenceFromThePattern(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	configure(t, e, "")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	template, source, err := e.pattern(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	p := newPlan(template, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "The documentation is updated"}})
	p.Stages[0].Checks = []plan.Check{{Aspect: "diff", Required: true, CompletionConditionIDs: []string{"C1"}, Target: "Documentation"}}
	p.Stages[0].Instructions = []string{"Keep the heading."}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	question := checkedResult(t, e)
	if question.Instruction.Question == nil || question.Instruction.Question.ApprovalTarget.Kind != form.ApprovalPlan {
		t.Fatalf("the first plan did not require approval: %+v", question)
	}
	for _, want := range []string{"Plan chain from implement", "definition implement, assignee worker", "required check diff (judge)", "Keep the heading.",
		"Structural difference from the cited pattern test", `"Field": "checks"`, `"wording"`, `"diff"`} {
		if !strings.Contains(question.Instruction.Question.Text, want) {
			t.Fatalf("plan approval omitted %q: %s", want, question.Instruction.Question.Text)
		}
	}
}

func TestOutsideNormChangeBeforeRoundRequiresLoopConfirmation(t *testing.T) {
	e, _, guides, _ := outsideSourceWork(t)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	round, _ := state.OpenRound()
	if err := e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "Continue in another round"}); err != nil {
		t.Fatal(err)
	}
	write(t, guides, "added.md", "A guide added outside the repository.\n")
	result := checkedNext(t, e)
	if result.Instruction.Question == nil || result.Instruction.Question.StopCategory != "meta_loop_edit" || !strings.Contains(result.Instruction.Question.Text, "guides/added.md") {
		t.Fatalf("an outside norm change did not require loop confirmation: %+v", result)
	}
}

func TestReadOnlySourceEditIsReturnedAsLesson(t *testing.T) {
	e, ctx := fixture(t)
	configure(t, e, "[sources.rules]\nwritable = false\n")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	work := checkedNext(t, e)
	write(t, e.Root(), ".devkit/rules.md", "# Improved read-only rules\n")
	submitWork(t, e, work)
	if _, err := e.Next(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	rounds := state.Rounds(state.StageRuns()[0].ID)
	decision := rounds[0].Decision
	if decision == nil || decision.Action != plan.Fix || !strings.Contains(decision.Reason, "Read-only sources were edited: .devkit/rules.md (source rules)") || !strings.Contains(decision.Reason, "devkit note lesson") {
		t.Fatalf("a read-only source edit was not returned for a lesson: %+v", decision)
	}
	if run, _ := state.LatestStageRun(); run.State == ledger.StageCompleted {
		t.Fatal("a read-only source edit completed the stage")
	}
}
