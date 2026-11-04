package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/check"
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
	write(t, e.Root(), "devkit.toml", "kit_version = 'v2'\n[approval]\nplan = 'deviation'\n"+settings)
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
	configureStage(t, e, func(stage *plan.Stage) {
		item := judgeCheck("wording", false)
		item.Criteria = append(item.Criteria, plan.Selection{Source: "principles", Select: "designing.md"})
		stage.Checks = []plan.Check{item}
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
	data, err := e.readInstruction(judgment.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Request.Criteria) != 2 || data.Request.Criteria[0].ID != "review/wording.md" || data.Request.Criteria[0].ContentHash == "" {
		t.Fatalf("request did not record the criteria identities: %+v", data.Request.Criteria)
	}
}

func TestEmptyCriteriaSelectionStopsBeforeJudging(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) {
		item := judgeCheck("wording", false)
		item.Criteria = append(item.Criteria, plan.Selection{Source: "principles", Select: "review/missing-*.md"})
		stage.Checks = []plan.Check{item}
	})
	submitWork(t, e, working(t, e, ctx))
	result, err := e.Next(ctx)
	if err == nil || !strings.Contains(err.Error(), "review/missing-*.md") || !strings.Contains(err.Error(), "matched no documents") {
		t.Fatalf("an empty criteria selection did not stop the judgment: %+v, %v", result, err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	for _, instruction := range state.UnacceptedInstructions() {
		if instruction.Kind == form.InstructionJudgment {
			t.Fatalf("a judgment was requested without criteria: %+v", instruction)
		}
	}
}

func TestDetectionRequestWithholdsExpectations(t *testing.T) {
	e, ctx := fixture(t)
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	detection, err := e.Next(ctx)
	if err != nil || detection.Instruction == nil || detection.Instruction.Kind != form.InstructionDetectionTest {
		t.Fatalf("expected detection: %+v, %v", detection, err)
	}
	text := briefText(t, detection.Instruction)
	spec, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/examples/plan/spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := check.ParseDetectionSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	withheld := []string{string(spec), `"expected"`, "violation", "conforming", "-history.md", "Work-history file", "spec.json", ".devkit/examples", "Repository requirements", "Other fixed norms"}
	for _, example := range parsed.Cases {
		withheld = append(withheld, example.File)
	}
	for _, value := range withheld {
		if strings.Contains(text, value) {
			t.Fatalf("detection request disclosed %q", value)
		}
	}
	criteria, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/principles/review/plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, string(criteria)) {
		t.Fatal("detection request omitted the selected criteria")
	}
	for _, example := range parsed.Cases {
		body, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/examples/plan", example.File))
		if err != nil {
			t.Fatal(err)
		}
		encoded := string(marshal(t, string(body)))
		if !strings.Contains(text, encoded[1:len(encoded)-1]) {
			t.Fatalf("detection request omitted the body of an example")
		}
	}
	published, err := os.ReadFile(strings.TrimSuffix(detection.Instruction.Brief, ".md") + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(published), `"expected"`) {
		t.Fatal("the published brief data disclosed expected judgments")
	}
}

func TestDetectionIdentityFollowsCriteriaAndTemplate(t *testing.T) {
	e, ctx := fixture(t)
	item := judgeCheck("plan", true)
	identity := func() check.DetectionIdentity {
		t.Helper()
		head := git(t, e.Root(), "rev-parse", "HEAD")
		input, _, err := e.detectionInput(ctx, head, item)
		if err != nil {
			t.Fatal(err)
		}
		return input.Identity
	}
	commit := func(name, suffix string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(e.Root(), name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, e.Root(), name, string(data)+suffix)
		git(t, e.Root(), "add", ".")
		git(t, e.Root(), "commit", "-qm", "Change "+name)
	}
	original := identity()
	commit(".devkit/principles/working.md", "\nUnselected principle change.\n")
	if identity() != original {
		t.Fatal("a principle outside the criteria changed the detection identity")
	}
	commit(".devkit/principles/review/plan.md", "\nRevised criteria.\n")
	criteria := identity()
	if criteria.CriteriaHash == original.CriteriaHash || criteria.TemplateHash != original.TemplateHash {
		t.Fatalf("criteria change was not isolated in the identity: %+v, %+v", original, criteria)
	}
	commit(".devkit/templates/brief/judgment.md.tmpl", "\n")
	template := identity()
	if template.TemplateHash == criteria.TemplateHash || template.CriteriaHash != criteria.CriteriaHash {
		t.Fatalf("template change was not isolated in the identity: %+v, %+v", criteria, template)
	}
	records := []check.DetectionTest{{Aspect: "plan", Runner: "devkit-judge", Identity: original, Passed: true}}
	for _, current := range []check.DetectionIdentity{criteria, template} {
		if len(check.MissingDetection([]check.DetectionRequirement{{Aspect: "plan", Runner: "devkit-judge", Identity: current}}, records)) != 1 {
			t.Fatal("a changed identity reused the earlier pass")
		}
	}
}

func TestUnselectedNormDocumentChangeRequiresLoopConfirmation(t *testing.T) {
	e, ctx := fixture(t)
	configure(t, e, "[detection]\nrequired = false\n")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
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
	configure(t, e, fmt.Sprintf("[detection]\nrequired = false\n[sources.requirements]\npath = %q\n[sources.guides]\npath = %q\nnorm = true\n", products, guides))
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(briefText(t, start.Instruction), "Initial outside requirements.") {
		t.Fatal("planner brief did not read the outside requirements")
	}
	planResult(t, e, start)
	return e, products, guides, checkedNext(t, e)
}

func sourceWrites(t *testing.T, e *Engine) []*ledger.SourceWrite {
	t.Helper()
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	writes := []*ledger.SourceWrite{}
	for _, record := range records {
		if write, ok := record.Payload.(*ledger.SourceWrite); ok {
			writes = append(writes, write)
		}
	}
	return writes
}

func TestOutsideSourceIsFixedEditedConfirmedAndWritten(t *testing.T) {
	e, products, guides, work := outsideSourceWork(t)
	ctx := t.Context()
	copy := e.editDirectory("requirements")
	text := briefText(t, work.Instruction)
	if !strings.Contains(text, copy) || !strings.Contains(text, "Initial outside requirements.") {
		t.Fatalf("work brief did not show the fixed requirements and their editable copy")
	}
	// A change in the original after fixing is not visible through the fixed copy.
	write(t, guides, "style.md", "Initial outside guide.\n")
	write(t, copy, "requirements.md", "Edited outside requirements.\n")
	write(t, e.editDirectory("guides"), "style.md", "Edited outside guide.\n")
	submitWork(t, e, work)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	if round.ReturnedTarget == nil || round.ReturnedTarget.Sources == "" || round.ReturnedTarget.Sources == round.StartTarget.Sources {
		t.Fatalf("target identity did not include the edited outside documents: %+v", round)
	}
	if got, _ := os.ReadFile(filepath.Join(products, "requirements.md")); string(got) != "Initial outside requirements.\n" {
		t.Fatal("the original changed before approval")
	}
	question := checkedNext(t, e)
	if question.Instruction.Question == nil || question.Instruction.Question.StopCategory != "meta_loop_edit" || !strings.Contains(question.Instruction.Question.Text, "guides/style.md") {
		t.Fatalf("an outside norm edit was not confirmed as a loop edit: %+v", question)
	}
	if question.Instruction.Question.ApprovalTarget.Sources != round.ReturnedTarget.Sources {
		t.Fatal("the approval does not point to the judged target identity")
	}
	answerResult(t, e, question, ledger.AnswerApproved)
	done, err := e.Next(ctx)
	if err != nil || done.Status != ledger.WorkCompleted {
		t.Fatalf("work did not complete after approval: %+v, %v", done, err)
	}
	for name, want := range map[string]string{filepath.Join(products, "requirements.md"): "Edited outside requirements.\n", filepath.Join(guides, "style.md"): "Edited outside guide.\n"} {
		if got, err := os.ReadFile(name); err != nil || string(got) != want {
			t.Fatalf("%s was not written: %q, %v", name, got, err)
		}
	}
	writes := sourceWrites(t, e)
	applied := map[string]*ledger.SourceWrite{}
	for _, write := range writes {
		if write.Status == ledger.SourceWriteApplied {
			applied[write.Source] = write
		}
	}
	if len(writes) != 4 || applied["requirements"] == nil || applied["guides"] == nil || applied["guides"].ApprovalID == "" || applied["guides"].BeforeID == "" || applied["guides"].Target != *round.ReturnedTarget {
		t.Fatalf("writes were not recorded with their approval and before records: %+v", writes)
	}
}

func TestOutsideSourceWriteRejectsChangedOriginal(t *testing.T) {
	e, products, _, work := outsideSourceWork(t)
	ctx := t.Context()
	write(t, e.editDirectory("requirements"), "requirements.md", "Worker requirements.\n")
	write(t, products, "requirements.md", "Concurrent outside requirements.\n")
	submitWork(t, e, work)
	fix := checkedNext(t, e)
	if fix.Instruction.Kind != form.InstructionWork {
		t.Fatalf("a conflicting write did not return the work for review: %+v", fix)
	}
	if got, _ := os.ReadFile(filepath.Join(products, "requirements.md")); string(got) != "Concurrent outside requirements.\n" {
		t.Fatalf("a conflicting write replaced the changed original: %q", got)
	}
	writes := sourceWrites(t, e)
	if len(writes) != 2 || writes[1].Status != ledger.SourceWriteConflict {
		t.Fatalf("the conflict was not recorded: %+v", writes)
	}
	if !strings.Contains(briefText(t, fix.Instruction), "requirements/requirements.md") {
		t.Fatal("the fix round did not name the changed document")
	}
	submitWork(t, e, fix)
	done, err := e.Next(ctx)
	if err != nil || done.Status != ledger.WorkCompleted {
		t.Fatalf("reviewed edit did not complete: %+v, %v", done, err)
	}
	if got, _ := os.ReadFile(filepath.Join(products, "requirements.md")); string(got) != "Worker requirements.\n" {
		t.Fatalf("reviewed edit was not written against the current original: %q", got)
	}
	if _, err := os.Stat(filepath.Join(products, "requirements.md")); errors.Is(err, os.ErrNotExist) {
		t.Fatal("requirements were deleted")
	}
}

func TestPlanApprovalShowsChangedCriteriaSelections(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	configure(t, e, "[detection]\nrequired = false\n")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	template, source, err := e.template(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	p := plan.NewPlan(template, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "The documentation is updated"}})
	p.Stages[0].Checks[0].CompletionConditionIDs = []string{"C1"}
	p.Stages[0].Checks[0].Target = "Documentation"
	p.Stages[0].Checks[0].Criteria = []plan.Selection{{Source: "principles", Select: "review/consistency.md"}}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	question := checkedNext(t, e)
	if question.Instruction.Question == nil || question.Instruction.Question.ApprovalTarget.Kind != form.ApprovalPlan {
		t.Fatalf("a changed criteria selection did not require plan approval: %+v", question)
	}
	for _, want := range []string{"changed_criteria", "review/wording.md", "review/consistency.md"} {
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
	configure(t, e, "[detection]\nrequired = false\n[sources.rules]\nwritable = false\n")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
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
