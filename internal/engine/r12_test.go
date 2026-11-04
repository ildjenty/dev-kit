package engine

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/external"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestRoundLimitRejectionWaitsForHumanDecision(t *testing.T) {
	for _, interpretation := range []ledger.Interpretation{ledger.AnswerRejected, ledger.AnswerText} {
		t.Run(string(interpretation), func(t *testing.T) {
			e, ctx := fixture(t)
			work := flowWork(t, e, sessionWorker+"[commands.fail]\nexecutable = '/usr/bin/false'\ntimeout_seconds = 5\n", func(stage *plan.Stage) {
				stage.Checks = []plan.Check{{Aspect: "command:fail", Required: true}}
				stage.Limits = plan.Limits{MaxRounds: 1, MaxNoProgress: 1}
			}, nil)
			submitWork(t, e, work)
			asked := nextResult(t, e, ctx)
			if asked.Instruction == nil || asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != "round_limit" {
				t.Fatalf("expected a round limit question: %+v", asked)
			}
			q := asked.Instruction.Question
			observeAnswer(t, e, "Stop and wait for a new decision.")
			if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: interpretation, Text: "Wait"}); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				result := nextResult(t, e, ctx)
				status, err := e.Status(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if result.Instruction != nil || !strings.Contains(result.Message, "not approved") || status.Metrics.TotalRounds != 1 || status.Position.StageRun.State != ledger.StageAnswerWait {
					t.Fatalf("declined continuation advanced: %+v, %+v", result, status)
				}
			}
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			previous := state.AnsweredQuestions()[q.ID]
			observeAnswer(t, e, "Approve one more round.")
			if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerApproved, Supersedes: previous.RecordID}); err != nil {
				t.Fatal(err)
			}
			continued := nextResult(t, e, ctx)
			if continued.Instruction == nil || continued.Instruction.Kind != form.InstructionWork {
				t.Fatalf("approval did not continue: %+v", continued)
			}
		})
	}
}

func TestInvalidSnapshotConfigurationReturnsWorkerFix(t *testing.T) {
	for name, invalid := range map[string]string{"parse": "[broken", "validation": "kit_version = 'v3'\n[parallel]\nmax_concurrent = 0\n"} {
		t.Run(name, func(t *testing.T) {
			e, ctx := fixture(t)
			work := flowWork(t, e, sessionWorker, nil, nil)
			original, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
			if err != nil {
				t.Fatal(err)
			}
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			first, _ := state.OpenRound()
			write(t, e.Root(), "devkit.toml", invalid)
			submitWork(t, e, work)
			fix := nextResult(t, e, ctx)
			if fix.Instruction == nil || fix.Instruction.Kind != form.InstructionWork {
				t.Fatalf("invalid configuration did not return a worker: %+v", fix)
			}
			body := briefText(t, fix.Instruction)
			if !strings.Contains(body, "Fix devkit.toml") || !strings.Contains(body, "last valid norms") {
				t.Fatal("fix omitted the configuration error and file")
			}
			state, err = e.load()
			if err != nil {
				t.Fatal(err)
			}
			round, _ := state.OpenRound()
			if roundReference(round) != roundReference(first) || !slices.Equal(round.Norms, first.Norms) {
				t.Fatal("invalid configuration was applied")
			}
			write(t, e.Root(), "devkit.toml", string(original))
			submitWork(t, e, fix)
			completed := nextResult(t, e, ctx)
			if completed.Status != ledger.WorkCompleted {
				t.Fatalf("repaired work did not complete: %+v", completed)
			}
		})
	}
}

func TestCompletedWorkExternalApprovalAndIdempotency(t *testing.T) {
	for _, declared := range []bool{false, true} {
		t.Run(map[bool]string{false: "individual", true: "plan"}[declared], func(t *testing.T) {
			e, ctx := fixture(t)
			write(t, e.Root(), "publish", "#!/bin/sh\ncat >/dev/null\nprintf x >> .devkit/run/published\nprintf '%s' '{\"status\":\"succeeded\",\"result\":\"Published\"}'\n")
			if err := os.Chmod(filepath.Join(e.Root(), "publish"), 0755); err != nil {
				t.Fatal(err)
			}
			request := external.Request{Kind: "publish", Target: "fixture/item", Input: json.RawMessage(`{"message":"Hello"}`)}
			work := flowWork(t, e, sessionWorker+"[external.publish]\nexecutable = './publish'\ntimeout_seconds = 5\n", nil, func(p *plan.Plan) {
				if !declared {
					return
				}
				settings, err := e.settings(ctx, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				target, err := external.New(e.Root(), e.store(), e.validator, settings.External).Describe(request)
				if err != nil {
					t.Fatal(err)
				}
				p.ExternalOperations = []plan.ExternalOperation{{Kind: target.Kind, Target: target.Target, InputHash: target.InputHash, DefinitionHash: target.DefinitionHash, ExecutableHash: target.ExecutableHash}}
			})
			submitWork(t, e, work)
			if done := nextResult(t, e, ctx); done.Status != ledger.WorkCompleted {
				t.Fatalf("expected completion: %+v", done)
			}
			result, err := e.External(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !declared {
				if result.Instruction == nil || result.Instruction.Question == nil {
					t.Fatalf("completed external did not ask: %+v", result)
				}
				repeated := nextResult(t, e, ctx)
				if repeated.Instruction == nil || repeated.Instruction.ID != result.Instruction.ID {
					t.Fatal("next lost the external approval question")
				}
				answerResult(t, e, result, ledger.AnswerApproved)
				result, err = e.External(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
			}
			if result.External == nil || result.External.Operation.Status != ledger.ExternalSucceeded {
				t.Fatalf("external failed: %+v", result)
			}
			if _, err := e.External(ctx, request); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(e.runRoot(), "published"))
			if err != nil || string(data) != "x" {
				t.Fatalf("external was not executed exactly once: %q, %v", data, err)
			}
			if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err == nil {
				t.Fatal("completed work accepted a new submission")
			}
		})
	}
}

func TestPlannerSuppliesPatternHashesAndCheckDescriptions(t *testing.T) {
	e, ctx := fixture(t)
	configureCatalogEntry(t, e, "wording", "description = 'Review wording with the configured summary.'")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	body := briefText(t, start.Instruction)
	data, err := e.readInstruction(start.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test", "docs", "fix", "default"} {
		source, err := e.asset(ctx, data.Reference, "plans/"+name+".json")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body, "plans/"+name+".json:") || !strings.Contains(body, "content hash: "+plan.ContentHash(source)) {
			t.Fatalf("planner omitted pattern path or hash for %s", name)
		}
	}
	if !strings.Contains(body, "Review wording with the configured summary.") || !strings.Contains(body, "Runs command:fail.") {
		t.Fatal("configured or fallback check description missing")
	}
	// The planner can copy a non-prioritized pattern hash and submit it unchanged.
	source, err := e.asset(ctx, data.Reference, "plans/docs.json")
	if err != nil {
		t.Fatal(err)
	}
	pattern, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	p := newPlan(pattern, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "The documentation is updated"}})
	for i := range p.Stages {
		for j := range p.Stages[i].Checks {
			p.Stages[i].Checks[j].Target = "Documentation"
			p.Stages[i].Checks[j].CompletionConditionIDs = []string{"C1"}
		}
	}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	if _, err := e.Next(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.PlanEvaluations(nil)) != 1 {
		t.Fatal("copied non-prioritized hash was rejected")
	}
}

func TestEveryRequestKindHonorsByteLimit(t *testing.T) {
	e, ctx := fixture(t)
	work := flowWork(t, e, sessionWorker+"[briefs]\nmax_bytes = 600\n"+judgeEntry("wording", "brief_by_planner = true"), func(stage *plan.Stage) { stage.Checks = []plan.Check{judgeCheck("wording", false)} }, nil)
	submitWork(t, e, work)
	planned := nextResult(t, e, ctx)
	if planned.Instruction == nil || planned.Instruction.Kind != form.InstructionJudgmentBrief {
		t.Fatalf("expected pre-judgment: %+v", planned)
	}
	collect(t, e, planned.Instruction, map[string]any{"instruction_id": planned.Instruction.ID, "instructions": []string{"Review the heading."}})
	judged := nextResult(t, e, ctx)
	if judged.Instruction == nil || judged.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("expected judgment: %+v", judged)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[form.InstructionKind]bool{}
	for _, record := range records {
		meta, ok := record.Payload.(*ledger.Instruction)
		if !ok {
			continue
		}
		body, err := e.instructionBrief(meta.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > 600 {
			t.Fatalf("%s exceeded max_bytes: %d", meta.Kind, len(body))
		}
		if meta.Context == nil || meta.Context.BriefBytes != len(body) {
			t.Fatalf("%s omitted final size", meta.Kind)
		}
		kinds[meta.Kind] = true
	}
	for _, kind := range []form.InstructionKind{form.InstructionPlan, form.InstructionPlanCheck, form.InstructionWork, form.InstructionJudgmentBrief, form.InstructionJudgment, form.InstructionQuestion} {
		if !kinds[kind] {
			t.Fatalf("missing request kind %s", kind)
		}
	}
}

func TestMinimalRequestCannotFitIsConfigurationError(t *testing.T) {
	e, ctx := fixture(t)
	configure(t, e, "[briefs]\nmax_bytes = 1\n")
	_, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err == nil || !strings.Contains(err.Error(), "configuration error: [briefs] max_bytes = 1 cannot fit the minimal request") {
		t.Fatalf("missing configuration error: %v", err)
	}
}

func TestInitWritesCheckDescriptions(t *testing.T) {
	e, _ := fixture(t)
	data, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	settings, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for aspect, check := range settings.Checks {
		if aspect != "plan" && aspect != "command:fail" && aspect != "command:verify" && check.Description == "" {
			t.Fatalf("default check %s omitted description", aspect)
		}
	}
}

func TestBackInvalidatesAdoptedExecutionPathAndDefinitionOutputs(t *testing.T) {
	for _, backTarget := range []string{"purpose", "implement-1"} {
		t.Run(backTarget, func(t *testing.T) {
			e, ctx := fixture(t)
			work := flowWork(t, e, sessionWorker+"[[loop.path]]\ndefinition = 'implement'\nfollows = 'verification'\n", nil, func(p *plan.Plan) {
				p.StartStage = "purpose"
				p.Stages = []plan.Stage{instance("purpose", "purpose", doneTo("replan"))}
			})
			if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"purpose-output-marker"}})); err != nil {
				t.Fatal(err)
			}
			planning := nextResult(t, e, ctx)
			if planning.Instruction == nil || planning.Instruction.Kind != form.InstructionPlan {
				t.Fatalf("expected replan: %+v", planning)
			}
			impl1 := instance("implement-1", "implement", doneTo("implement-2"))
			impl2 := instance("implement-2", "implement", doneTo("verification-1"))
			verification := instance("verification-1", "verification", doneTo("edit"))
			verification.Outputs = []string{"implement"}
			verification.Transitions.Back = backTarget
			edit := instance("edit", "edit", nil)
			edit.Transitions.Back = backTarget
			p := composedPlan(t, e, 2, impl1, impl2, verification, edit)
			p.Stages = []plan.Stage{edit, verification, impl2, impl1}
			collect(t, e, planning.Instruction, form.PlannerResult{InstructionID: planning.Instruction.ID, Plan: marshal(t, p)})
			next := roleNext(t, e)
			for _, marker := range []string{"first-implementation-marker", "second-implementation-marker"} {
				if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: next.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{marker}})); err != nil {
					t.Fatal(err)
				}
				next = roleNext(t, e)
			}
			body := briefText(t, next.Instruction)
			for _, marker := range []string{"first-implementation-marker", "second-implementation-marker"} {
				if !strings.Contains(body, marker) {
					t.Fatalf("definition output selection omitted %s", marker)
				}
			}
			if strings.Contains(body, "purpose-output-marker") {
				t.Fatal("definition output selection included unrelated output")
			}
			if err := os.Remove(next.Instruction.Brief); err != nil {
				t.Fatal(err)
			}
			restored, err := e.instructionBrief(next.Instruction.ID)
			if err != nil || string(restored) != body {
				t.Fatalf("definition outputs did not survive regeneration: %v", err)
			}
			if backTarget != "purpose" {
				submitWork(t, e, next)
				next = roleNext(t, e)
			}
			if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: next.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Return to the earlier work?")})); err != nil {
				t.Fatal(err)
			}
			asked := nextResult(t, e, ctx)
			answerResult(t, e, asked, ledger.AnswerBack)
			planning = nextResult(t, e, ctx)
			if planning.Instruction == nil || planning.Instruction.Kind != form.InstructionPlan {
				t.Fatalf("expected back-triggered planner: %+v", planning)
			}
			body = briefText(t, planning.Instruction)
			for _, marker := range []string{"first-implementation-marker", "second-implementation-marker"} {
				if strings.Contains(body, marker) {
					t.Fatalf("back kept superseded output %s", marker)
				}
			}
			if strings.Contains(body, "purpose-output-marker") != (backTarget != "purpose") {
				t.Fatal("back invalidated the wrong adopted prefix")
			}
			// Old implement instances must not contribute a follows violation to the new path.
			p = composedPlan(t, e, 3, instance(backTarget, map[string]string{"purpose": "purpose", "implement-1": "implement"}[backTarget], nil))
			if backTarget == "implement-1" {
				p.Stages[0].Transitions.Done = doneTo("verification-2")
				p.Stages = append(p.Stages, instance("verification-2", "verification", nil))
			}
			collect(t, e, planning.Instruction, form.PlannerResult{InstructionID: planning.Instruction.ID, Plan: marshal(t, p)})
			resumed := roleNext(t, e)
			if resumed.Instruction == nil || resumed.Instruction.Kind != form.InstructionWork {
				t.Fatalf("superseded path blocked replan: %+v", resumed)
			}
		})
	}
}

func TestBoundedJudgeCommandProcess(t *testing.T) {
	if os.Getenv("DEVKIT_R12_COMMAND") != "1" {
		return
	}
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	var pointer struct {
		RequestPath string `json:"request_path"`
	}
	if err := json.Unmarshal(body, &pointer); err != nil {
		os.Exit(3)
	}
	size := len(body)
	if pointer.RequestPath != "" {
		body, err = os.ReadFile(pointer.RequestPath)
		if err != nil {
			os.Exit(4)
		}
	}
	var input capturedJudgeInput
	if err := json.Unmarshal(body, &input); err != nil {
		os.Exit(5)
	}
	capture := filepath.Join(".devkit/run/work", os.Getenv("DEVKIT_WORK"), "command-"+string(input.Kind)+".json")
	if err := os.WriteFile(capture, marshalCommandCapture(size, pointer.RequestPath, string(body)), 0644); err != nil {
		os.Exit(6)
	}
	if err := json.NewEncoder(os.Stdout).Encode(pass(input.Request)); err != nil {
		os.Exit(7)
	}
	os.Exit(0)
}

func marshalCommandCapture(size int, path, body string) []byte {
	data, _ := json.Marshal(struct {
		Size       int
		Path, Body string
	}{size, path, body})
	return data
}

func TestRegisteredCommandRequestKindsHonorByteLimit(t *testing.T) {
	e, ctx := fixture(t)
	t.Setenv("DEVKIT_R12_COMMAND", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "judge-schema.json", string(e.schemas["judgment"]))
	configure(t, e, "[briefs]\nmax_bytes = 600\n[plan_check]\nrunner = 'capture'\n[judges.capture]\nexecutable = "+strconv.Quote(executable)+"\nargs = ['-test.run=^TestBoundedJudgeCommandProcess$']\ntimeout_seconds = 5\noutput_schema = 'judge-schema.json'\n[checks.wording]\nrunner = 'capture'\ncriteria = [{source='principles',select='review/wording.md'}]\n")
	configureStage(t, e, func(stage *plan.Stage) { stage.Checks = []plan.Check{judgeCheck("wording", false)} })
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	work := approvedNext(t, e, nextResult(t, e, ctx))
	if work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("bounded command did not reach work: %+v", work)
	}
	submitWork(t, e, work)
	done := nextResult(t, e, ctx)
	if done.Status != ledger.WorkCompleted {
		t.Fatalf("bounded command did not complete: %+v", done)
	}
	for _, kind := range []check.RequestKind{check.PlanRequest, check.RoundRequest} {
		body, err := os.ReadFile(filepath.Join(e.workRoot(), "command-"+string(kind)+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var capture struct {
			Size       int
			Path, Body string
		}
		if err := json.Unmarshal(body, &capture); err != nil {
			t.Fatal(err)
		}
		if capture.Size > 600 || capture.Path == "" || !strings.Contains(capture.Body, "criteria_bodies") || !strings.Contains(capture.Body, "OutputSchema") {
			t.Fatalf("%s input was not preserved within limit: %+v", kind, capture)
		}
	}
}

func TestWorkerWorktreeReturnsBrokenConfigurationForRepairWithinLimit(t *testing.T) {
	e, ctx := defaultFixture(t)
	work := flowWork(t, e, "[briefs]\nmax_bytes = 600\n", nil, nil)
	if len(work.Instructions) != 1 {
		t.Fatalf("expected one unit work instruction: %+v", work)
	}
	unit := work.Instructions[0]
	if len(briefText(t, &unit)) > 600 {
		t.Fatal("unit request exceeded max_bytes")
	}
	write(t, unit.TargetPath, "devkit.toml", "[broken")
	collect(t, e, &unit, form.Progress{InstructionID: unit.ID, TerminationKind: form.TerminationCompleted})
	fix := nextResult(t, e, ctx)
	if len(fix.Instructions) != 1 || fix.Instructions[0].Kind != form.InstructionUnitWork || fix.Instructions[0].ID == unit.ID {
		t.Fatalf("broken merged configuration did not return to the worker: %+v", fix)
	}
	data, err := e.readInstruction(fix.Instructions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if data.Context == nil || !strings.Contains(data.Context.PreCommitOutput, "Fix devkit.toml") || !strings.Contains(data.Context.PreCommitOutput, "parse configuration") {
		t.Fatal("worker repair omitted the parse error and file")
	}
}
