package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

// checkedNext passes checks and approves plans until another instruction is ready.
func checkedNext(t *testing.T, e *Engine) Result {
	t.Helper()
	result := checkedResult(t, e)
	for result.Instruction.Question != nil && result.Instruction.Question.StopCategory == "plan_approval" {
		answerResult(t, e, result, ledger.AnswerApproved)
		result = checkedResult(t, e)
	}
	return result
}

// checkedResult passes checks until another instruction, such as a plan approval, is ready.
func checkedResult(t *testing.T, e *Engine) Result {
	t.Helper()
	for i := 0; i < 40; i++ {
		result, err := e.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if result.Instruction == nil {
			t.Fatalf("missing instruction: %+v", result)
		}
		switch result.Instruction.Kind {
		case form.InstructionPlanCheck, form.InstructionJudgment:
			data, err := e.readInstruction(result.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			collect(t, e, result.Instruction, pass(*data.Request))
		default:
			return result
		}
	}
	t.Fatal("checks did not terminate")
	return Result{}
}

func answerResult(t *testing.T, e *Engine, result Result, interpretation ledger.Interpretation) {
	t.Helper()
	if result.Instruction == nil || result.Instruction.Question == nil {
		t.Fatalf("expected a question: %+v", result)
	}
	observeAnswer(t, e, "Use the recorded human decision.")
	if _, err := e.Answer(t.Context(), AnswerRequest{QuestionID: result.Instruction.Question.ID, Interpretation: interpretation, Text: "Use the recorded human decision"}); err != nil {
		t.Fatal(err)
	}
}

func TestOutOfRoundNormsUseCommittedHEAD(t *testing.T) {
	e, ctx := fixture(t)
	started, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, started)
	write(t, e.Root(), ".devkit/rules.md", "Unconfirmed working rule marker\n")
	originalConfig, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", "Invalid uncommitted configuration")
	e, err = newTestEngine(ctx, e.Root(), e.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	review, err := e.Next(ctx)
	if err != nil || review.Instruction.Kind != form.InstructionPlanCheck {
		t.Fatalf("plan check: %+v, %v", review, err)
	}
	head := git(t, e.Root(), "rev-parse", "HEAD")
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil || data.Reference != head {
		t.Fatalf("plan check did not use HEAD: %+v, %v", data, err)
	}
	body, err := e.instructionBrief(review.Instruction.ID)
	if err != nil || strings.Contains(string(body), "Unconfirmed working rule marker") {
		t.Fatalf("plan check read working norms: %v", err)
	}
	write(t, e.Root(), "devkit.toml", string(originalConfig))
	collect(t, e, review.Instruction, pass(*data.Request))
	question, err := e.Next(ctx)
	question = approvedNext(t, e, question)
	if err != nil || question.Instruction.Question == nil || question.Instruction.Question.StopCategory != "meta_loop_edit" {
		t.Fatalf("uncommitted rule became effective: %+v, %v", question, err)
	}
	if !strings.Contains(question.Instruction.Question.Text, workStartBase+":") {
		t.Fatalf("loop edit question without an earlier approval did not name the work start as its base:\n%s", question.Instruction.Question.Text)
	}
}

func TestCommittedNormChangeReissuesPendingPlanCheck(t *testing.T) {
	e, ctx := fixture(t)
	write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n[approval]\nplan = 'deviation'\n"))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure deviation approval")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, review.Instruction, pass(*data.Request))
	write(t, e.Root(), ".devkit/rules.md", "# Revised committed rules\n")
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Change committed norms")
	next, err := e.Next(ctx)
	if err != nil || next.Instruction.Kind != form.InstructionPlanCheck || next.Instruction.ID == review.Instruction.ID {
		t.Fatalf("stale check accepted: %+v, %v", next, err)
	}
}

func TestGateQuestionContainsRecordedEvidence(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(s *plan.Stage) {
		s.Gate = plan.Gate{Required: true, Reason: "Confirm the published documentation scope"}
		s.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Evidence target\n")
	progress := form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Recorded work performed"}, Confirmed: []string{"Recorded confirmed matter"}, Unconfirmed: []string{"Recorded unconfirmed matter"}}
	if _, err := e.Submit(ctx, marshal(t, progress)); err != nil {
		t.Fatal(err)
	}
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	serialized := briefText(t, review.Instruction)
	for _, want := range []string{"Comparison base (tree at the start of this stage run):", "Changed files:", "README.md", "Diff file:", "-history.md"} {
		if !strings.Contains(serialized, want) {
			t.Fatalf("judgment omitted %q", want)
		}
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	patch, err := os.ReadFile(filepath.Join(e.workRoot(), "brief", round.StartTarget.Tree+"-"+round.ReturnedTarget.Tree+".diff"))
	if err != nil || !strings.Contains(string(patch), "+# Evidence target") {
		t.Fatalf("missing patch: %q, %v", patch, err)
	}
	collect(t, e, review.Instruction, pass(*data.Request))
	asked, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{progress.WorkPerformed[0], progress.Confirmed[0], progress.Unconfirmed[0], "Confirm the published documentation scope", "Diff file:"} {
		if asked.Instruction.Question == nil || !strings.Contains(asked.Instruction.Question.Text, want) {
			t.Fatalf("gate omitted %q: %+v", want, asked)
		}
	}
}

func TestBackAnswerAndReplanApprovalConditions(t *testing.T) {
	e, ctx := fixture(t)
	write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n"))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Require requirements approval")
	started, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	p := planResult(t, e, started)
	approved := checkedResult(t, e)
	if approved.Instruction.Question == nil || approved.Instruction.Question.StopCategory != "plan_approval" {
		t.Fatalf("missing initial approval: %+v", approved)
	}
	answerResult(t, e, approved, ledger.AnswerApproved)
	work := checkedNext(t, e)
	for _, changed := range []bool{false, true} {
		if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Confirm how to continue."), Obstacles: []string{"Reconsider the earlier stage"}})); err != nil {
			t.Fatal(err)
		}
		asked := checkedNext(t, e)
		answerResult(t, e, asked, ledger.AnswerBack)
		state, err := e.load()
		if err != nil {
			t.Fatal(err)
		}
		run, _ := state.LatestStageRun()
		rounds := state.Rounds(run.ID)
		if rounds[len(rounds)-1].Decision.Action != plan.Back || run.State != ledger.StageInvalidated {
			t.Fatal("back answer did not record the back route")
		}
		replanning := checkedNext(t, e)
		if replanning.Instruction.Kind != form.InstructionPlan {
			t.Fatalf("back did not replan: %+v", replanning)
		}
		p.Version++
		if changed {
			p.CompletionConditions[0].Description = "The installation documentation is complete"
		}
		collect(t, e, replanning.Instruction, form.PlannerResult{InstructionID: replanning.Instruction.ID, Plan: marshal(t, p)})
		result := checkedResult(t, e)
		if changed {
			if result.Instruction.Question == nil || result.Instruction.Question.StopCategory != "plan_approval" {
				t.Fatalf("changed conditions bypassed approval: %+v", result)
			}
			answerResult(t, e, result, ledger.AnswerApproved)
			work = checkedNext(t, e)
		} else {
			if result.Instruction.Kind != form.InstructionWork {
				t.Fatalf("unchanged conditions required approval: %+v", result)
			}
			work = result
		}
	}
}

func TestOpenQuestionsRequireApprovalUnderDeviationPolicy(t *testing.T) {
	e, ctx := fixture(t)
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := e.asset(ctx, "HEAD", "plans/test.json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	p := newPlan(template, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "Documentation is updated"}})
	p.OpenQuestions = []string{"Which supported platforms must be documented?"}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	asked := checkedResult(t, e)
	if asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != "plan_approval" || !strings.Contains(asked.Instruction.Question.Text, p.OpenQuestions[0]) {
		t.Fatalf("open questions bypassed the human: %+v", asked)
	}
	answerResult(t, e, asked, ledger.AnswerRejected)
	revised := checkedNext(t, e)
	if revised.Instruction.Kind != form.InstructionPlan {
		t.Fatalf("rejection did not return to planner: %+v", revised)
	}
}

func TestRegisteredPlanJudgeProcess(t *testing.T) {
	if os.Getenv("DEVKIT_WORK") == "" {
		return
	}
	var request check.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		os.Exit(2)
	}
	if request.Aspect.ID != "consistency" {
		os.Exit(3)
	}
	if err := json.NewEncoder(os.Stdout).Encode(pass(request)); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestPlanCheckUsesConfiguredCommandRunner(t *testing.T) {
	e, ctx := fixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "judge-schema.json", string(e.schemas["judgment"]))
	settings := withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n[approval]\nplan = 'deviation'\n[plan_check]\naspect = 'consistency'\nrunner = 'registered-plan'\n[judges.registered-plan]\nexecutable = '" + executable + "'\nargs = ['-test.run=^TestRegisteredPlanJudgeProcess$']\ntimeout_seconds = 5\noutput_schema = 'judge-schema.json'\n")
	write(t, e.Root(), "devkit.toml", settings)
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure plan judgment command")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	result, err := e.Next(ctx)
	result = approvedNext(t, e, result)
	if err != nil || result.Instruction == nil || result.Instruction.Kind != form.InstructionWork {
		t.Fatalf("plan command was not executed: %+v, %v", result, err)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if j, ok := record.Payload.(*ledger.Judgment); ok && j.TargetVersion.Kind == form.TargetPlan {
			if j.Aspect != "consistency" || j.Runner != "registered-plan" || j.Provenance.Source != check.JudgeCommand {
				t.Fatalf("wrong plan judgment: %+v", j)
			}
			return
		}
	}
	t.Fatal("plan command judgment was not recorded")
}

func TestRemainingPlanAfterReplanSurvivesRevision(t *testing.T) {
	e, ctx := fixture(t)
	source, err := e.asset(ctx, "HEAD", "plans/test.json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	base := template.Stages[0]
	makeStage := func(id string, done *string) plan.Stage {
		s := base
		s.ID = id
		s.Transitions = plan.Transitions{Fix: id, Ask: id, Back: id, Done: done}
		return s
	}
	scope, implement, replan := "scope", "implement", "replan"
	template.Stages = []plan.Stage{makeStage("purpose", &replan), makeStage(scope, &implement), makeStage(implement, nil)}
	template.StartStage = "purpose"
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure replanning order")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	// A plan ends its done path at replan; the stages after it belong to the next plan.
	p := planResultWith(t, e, start, func(p *plan.Plan) { p.Stages = p.Stages[:1] })
	work := checkedNext(t, e)
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	planning := checkedNext(t, e)
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	var expected *plan.Plan
	for _, record := range records {
		if i, ok := record.Payload.(*ledger.Instruction); ok && i.ID == planning.Instruction.ID {
			expected = i.Context.RemainingPlan
		}
	}
	if expected == nil || len(expected.Stages) != 0 {
		t.Fatalf("a replan left stages in the remaining plan: %+v", expected)
	}
	p.Version = 2
	p.Stages, p.StartStage = template.Stages[1:], scope
	collect(t, e, planning.Instruction, form.PlannerResult{InstructionID: planning.Instruction.ID, Plan: marshal(t, p)})
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	failed := pass(*data.Request)
	failed.Judgment = form.Fail
	failed.Findings = []form.Finding{{ID: "revise-plan", Blocking: true, Kind: form.AutoFixable, Scenario: "Preserve the remaining stage order"}}
	collect(t, e, review.Instruction, failed)
	revised, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	records, err = e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if i, ok := record.Payload.(*ledger.Instruction); ok && i.ID == revised.Instruction.ID {
			if i.Context == nil || !reflect.DeepEqual(expected, i.Context.RemainingPlan) {
				t.Fatal("revision recomputed remaining work from the rejected proposal")
			}
			return
		}
	}
	t.Fatal("revised planner instruction was not recorded")
}

func TestEmbeddedSchemasAreSelfContained(t *testing.T) {
	e, ctx := fixture(t)
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"planner-result", "judgment", "decision", "evidence", "progress", "structure"} {
		source, err := e.asset(ctx, "HEAD", "schemas/"+name+".json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := form.CompileSchema(source); err != nil {
			t.Fatalf("%s schema needs external resources: %v", name, err)
		}
	}
	p := planResult(t, e, start)
	// The planner output validates with only the schema embedded in its brief.
	source, err := e.asset(ctx, "HEAD", "schemas/planner-result.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := form.CompileSchema(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(marshal(t, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredProductsAndMissingRules(t *testing.T) {
	e, ctx := fixture(t)
	write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n[approval]\nplan = 'deviation'\n[sources.requirements]\npath = 'docs/accepted-requirements.md'\n[sources.structure]\npath = 'docs/system-structure.json'\n"))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure product paths")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := e.instructionBrief(start.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"not created yet; create it at docs/accepted-requirements.md", "structure.json -> docs/system-structure.json (not created yet)"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("planner omitted %q", want)
		}
	}
	planResult(t, e, start)
	work := checkedNext(t, e)
	body, err = e.instructionBrief(work.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"not created yet; create it at docs/accepted-requirements.md", "structure.json -> docs/system-structure.json (not created yet)", "Supported `devkit note` kinds: `decision`, `evidence`, `lesson`", "### structure JSON Schema"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("work brief omitted %q", want)
		}
	}
	if _, err := e.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	git(t, e.Root(), "rm", ".devkit/rules.md")
	git(t, e.Root(), "commit", "-qm", "Remove required rules")
	e.workID = ""
	if _, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"}); err == nil || !strings.Contains(err.Error(), "required rules file .devkit/rules.md") {
		t.Fatalf("missing rules accepted: %v", err)
	}
}

func TestWorkHistoryIsGenericAndFixedAtIssuance(t *testing.T) {
	e, ctx := fixture(t)
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	// The "all" scope selects every lesson and evidence record; narrower scopes are tested separately.
	template.Stages[0].History = plan.HistoryAll
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Select all history")
	working(t, e, ctx)
	if _, err := e.Note(ctx, ledger.KindLesson, marshal(t, ledger.Lesson{Content: "First recorded lesson", Evidence: []form.Reference{}})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Note(ctx, ledger.KindEvidence, marshal(t, form.Evidence{ID: "E1", Content: "Tests failed before the implementation", Sources: []form.Reference{{External: "test.log"}}, VerificationMethod: "Ran the tests before writing the code"})); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	run, _ := state.LatestStageRun()
	if err := e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "Use the previous lesson"}); err != nil {
		t.Fatal(err)
	}
	work := checkedNext(t, e)
	path := filepath.Join(e.workRoot(), "brief", work.Instruction.ID+"-history.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(original), "First recorded lesson") || !strings.Contains(string(original), "Tests failed before the implementation") {
		t.Fatal("work history lacked a lesson or recorded evidence")
	}
	if _, err := e.Note(ctx, ledger.KindLesson, marshal(t, ledger.Lesson{Content: "Later lesson must stay out", Evidence: []form.Reference{}})); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(e.workRoot(), "brief")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.instructionBrief(work.Instruction.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil || string(restored) != string(original) {
		t.Fatalf("work history changed after issuance: %v", err)
	}
}

func TestLoopApprovalPolicyIsIndependentOfRouterCategories(t *testing.T) {
	for _, policy := range []string{"confirm", "allow"} {
		for _, when := range []string{"before_round", "before_commit"} {
			t.Run(policy+"/"+when, func(t *testing.T) {
				e, ctx := fixture(t)
				write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[roles.worker]\nkind = 'work'\nagent = ''\n[approval]\nplan = 'deviation'\nmeta_loop_edit = '"+policy+"'\n[router]\nhuman_confirmation = []\n"))
				git(t, e.Root(), "add", ".")
				git(t, e.Root(), "commit", "-qm", "Configure loop approval")
				start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
				if err != nil {
					t.Fatal(err)
				}
				planResult(t, e, start)
				work := checkedNext(t, e)
				if when == "before_round" {
					state, err := e.load()
					if err != nil {
						t.Fatal(err)
					}
					run, _ := state.LatestStageRun()
					round, _ := state.OpenRound()
					if err := e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "Update the loop"}); err != nil {
						t.Fatal(err)
					}
				}
				write(t, e.Root(), ".devkit/rules.md", "# Changed loop rule\n")
				if when == "before_commit" {
					if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
						t.Fatal(err)
					}
				}
				result := checkedNext(t, e)
				if policy == "confirm" {
					if result.Instruction.Question == nil || result.Instruction.Question.StopCategory != "meta_loop_edit" {
						t.Fatalf("empty router categories bypassed loop approval: %+v", result)
					}
					for _, want := range []string{".devkit/rules.md", "Diff file:"} {
						if !strings.Contains(result.Instruction.Question.Text, want) {
							t.Fatalf("loop question omitted %q", want)
						}
					}
				} else if result.Instruction.Question != nil {
					t.Fatalf("allow policy still asked: %+v", result)
				}
			})
		}
	}
}
