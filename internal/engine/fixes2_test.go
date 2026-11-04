package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestDetectionIndexUsesInstructionReferenceOnAcceptAndRestore(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[state]\ndetect = '.devkit/fixed-detect.json'\n")
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure detection index path")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	detection, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(detection.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	settings, err = os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", strings.Replace(string(settings), ".devkit/fixed-detect.json", ".devkit/uncommitted-detect.json", 1))
	acceptDetectionResult(t, e, detection.Instruction)
	review, err := e.Next(ctx)
	if err != nil || review.Instruction.Kind != form.InstructionPlanCheck {
		t.Fatalf("result was not accepted with an uncommitted detect path: %+v, %v", review, err)
	}
	index := filepath.Join(e.Root(), ".devkit/fixed-detect.json")
	if _, err := os.Stat(index); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.Root(), ".devkit/uncommitted-detect.json")); !os.IsNotExist(err) {
		t.Fatalf("uncommitted detection path was used: %v", err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if meta, ok := record.Payload.(*ledger.Instruction); ok && meta.ID == detection.Instruction.ID {
			if err := os.Remove(index); err != nil {
				t.Fatal(err)
			}
			if _, _, err := e.restoreDetection(ctx, state, *meta, data); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(index); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("detection instruction missing")
}

func TestReplanNotAssessedHumanRevisionKeepsRemainingWork(t *testing.T) {
	e, ctx := fixture(t)
	source, err := e.asset(ctx, "HEAD", "plans/test.json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	purpose := template.Stages[0]
	replan := "replan"
	purpose.ID = "purpose"
	purpose.Transitions = plan.Transitions{Fix: "purpose", Ask: "purpose", Back: "purpose", Done: &replan}
	template.StartStage = purpose.ID
	template.Stages = append([]plan.Stage{purpose}, template.Stages...)
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure replanning")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	p := planResult(t, e, start)
	work := checkedNext(t, e)
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	planning := checkedNext(t, e)
	before, err := e.readInstruction(planning.Instruction.ID)
	if err != nil || before.Origin != plan.BackProposal || before.Context.RemainingPlan == nil {
		t.Fatalf("remaining work missing: %+v, %v", before, err)
	}
	p.Version = 2
	collect(t, e, planning.Instruction, form.PlannerResult{InstructionID: planning.Instruction.ID, Plan: marshal(t, p)})
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	judgment := pass(*request.Request)
	judgment.Judgment = form.NotAssessed
	collect(t, e, review.Instruction, judgment)
	asked, err := e.Next(ctx)
	if err != nil || asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != "plan_not_assessed" {
		t.Fatalf("expected revision question: %+v, %v", asked, err)
	}
	answerResult(t, e, asked, ledger.AnswerText)
	revised, err := e.Next(ctx)
	if err != nil || revised.Instruction.Kind != form.InstructionPlan {
		t.Fatalf("expected planner: %+v, %v", revised, err)
	}
	after, err := e.readInstruction(revised.Instruction.ID)
	if err != nil || after.Origin != plan.BackProposal || !reflect.DeepEqual(before.Context.RemainingPlan, after.Context.RemainingPlan) {
		t.Fatalf("human revision lost replan kind or remaining work: %+v, %v", after, err)
	}
}

func TestDetectionHistoryIncludesPassedAndFailedCases(t *testing.T) {
	for _, passed := range []bool{true, false} {
		t.Run(map[bool]string{true: "passed", false: "failed"}[passed], func(t *testing.T) {
			e, ctx := fixture(t)
			start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
			if err != nil {
				t.Fatal(err)
			}
			planResult(t, e, start)
			detection, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			data, err := e.readInstruction(detection.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			result := form.DetectionResult{InstructionID: detection.Instruction.ID}
			for _, example := range data.Detection.Spec.Cases {
				j := pass(*data.Request)
				j.Judgment = example.Expected
				if example.Kind == check.Violation {
					j.Findings = []form.Finding{{ID: "history-finding", Blocking: true, Kind: form.AutoFixable, Scenario: "Detection history finding"}}
					if !passed {
						j.Judgment, j.Findings = form.Pass, []form.Finding{}
					}
				} else {
					j.Findings = []form.Finding{{ID: "reference", Kind: form.AutoFixable, Scenario: "Detection history reference finding"}}
				}
				result.Observations = append(result.Observations, observed(example.ID, j, "Case assessment reason"))
			}
			collect(t, e, detection.Instruction, result)
			if _, err := e.Next(ctx); err != nil {
				t.Fatal(err)
			}
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			meta := ledger.Instruction{ID: "history"}
			path, err := e.historyIndex(state, &meta, historyScope{Kind: plan.HistoryAll}, nil)
			if err != nil {
				t.Fatal(err)
			}
			history, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"detection_test", `"cases"`, `"observation"`, `"expected"`, `"passed": ` + map[bool]string{true: "true", false: "false"}[passed], "Case assessment reason"} {
				if !strings.Contains(string(history), want) {
					t.Fatalf("history omitted %q", want)
				}
			}
			if !passed {
				q := state.UnansweredQuestions()[0]
				for _, want := range []string{"human must commit", "out-of-round", "not required", "[detection] required = false"} {
					if !strings.Contains(q.Text, want) {
						t.Fatalf("detection failure question omitted %q: %s", want, q.Text)
					}
				}
			}
		})
	}
}

func TestFixRoundDiffAndClassificationAreCumulative(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(s *plan.Stage) {
		s.Checks = []plan.Check{judgeCheck("diff", false)}
	})
	work := working(t, e, ctx)
	write(t, e.Root(), "go.mod", "module example.invalid/cumulative\n")
	write(t, e.Root(), ".agents/skills/example/SKILL.md", "# Cumulative loop edit\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
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
	failed := pass(*data.Request)
	failed.Judgment = form.Fail
	failed.Findings = []form.Finding{{ID: "fix", Blocking: true, Kind: form.AutoFixable, Scenario: "Fix the documentation"}}
	collect(t, e, review.Instruction, failed)
	asked, err := e.Next(ctx)
	if err != nil || asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != config.MetaLoopEdit {
		t.Fatalf("expected loop confirmation before next round: %+v, %v", asked, err)
	}
	answerResult(t, e, asked, ledger.AnswerApproved)
	work = checkedNext(t, e)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	rounds := state.Rounds(run.ID)
	write(t, e.Root(), "README.md", "# Second round\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	review, err = e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err = e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	serialized := briefText(t, review.Instruction)
	for _, want := range []string{"go.mod", ".agents/skills/example/SKILL.md", "README.md"} {
		if !strings.Contains(serialized, want) {
			t.Fatalf("second round diff omitted %q", want)
		}
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	patch, err := os.ReadFile(filepath.Join(e.workRoot(), "brief", rounds[0].StartTarget.Tree+"-"+round.ReturnedTarget.Tree+".diff"))
	if err != nil || !strings.Contains(string(patch), "+module example.invalid/cumulative") {
		t.Fatalf("first-round diff lost: %s, %v", patch, err)
	}
	settings, err := e.settings(ctx, roundReference(round))
	if err != nil {
		t.Fatal(err)
	}
	categories, err := e.roundConfirmationCategories(ctx, state, round, settings)
	// The dependency change from the failed round still counts; the loop edit was approved before
	// this round, so it does not ask again.
	if err != nil || !strings.Contains(strings.Join(categories, ","), config.DependencyChanged) || strings.Contains(strings.Join(categories, ","), config.MetaLoopEdit) {
		t.Fatalf("cumulative classification is wrong: %v, %v", categories, err)
	}
}

func TestMalformedSubagentResultsReissueAndEscalate(t *testing.T) {
	for _, mode := range []string{"schema", "content", "json"} {
		t.Run(mode, func(t *testing.T) {
			e, ctx := fixture(t)
			result, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
			if err != nil {
				t.Fatal(err)
			}
			initial, err := e.readInstruction(result.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 3; attempt++ {
				previous := result.Instruction
				message := "This is not JSON"
				if mode == "schema" {
					message = string(marshal(t, map[string]any{"instruction_id": previous.ID}))
				} else if mode == "content" {
					template, source, err := e.template(ctx, initial.Reference, "test")
					if err != nil {
						t.Fatal(err)
					}
					p := plan.NewPlan(template, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "Documentation is complete"}})
					p.Version = 99
					message = string(marshal(t, form.PlannerResult{InstructionID: previous.ID, Plan: marshal(t, p)}))
				}
				h := hook.Handler{Root: e.Root(), WorkID: e.workID, Validator: e.validator, InstructionRunner: e.InstructionRunner}
				if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.SubagentStop, SessionID: e.sessionID, AgentID: "malformed-agent", AgentType: previous.Runner, LastAssistantMessage: message}); err != nil {
					t.Fatal(err)
				}
				if attempt == 0 {
					state, err := e.load()
					if err != nil {
						t.Fatal(err)
					}
					if changed, err := e.acceptPending(ctx, state); err != nil || !changed {
						t.Fatalf("failure was not recorded: %v", err)
					}
					e, err = newTestEngine(ctx, e.Root(), e.sessionID)
					if err != nil {
						t.Fatal(err)
					}
				}
				result, err = e.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if attempt < 2 {
					if result.Instruction.Kind != form.InstructionPlan || result.Instruction.ID == previous.ID || result.Instruction.Reissue != attempt+1 {
						t.Fatalf("invalid result did not reissue: %+v", result)
					}
					reissued, err := e.readInstruction(result.Instruction.ID)
					if err != nil || reissued.Reference != initial.Reference || reissued.Origin != initial.Origin || !reflect.DeepEqual(reissued.Context, initial.Context) {
						t.Fatalf("reissue changed fixed inputs: %+v, %v", reissued, err)
					}
					brief, err := os.ReadFile(result.Instruction.Brief)
					if err != nil || !strings.Contains(string(brief), "## Previous result rejected") {
						t.Fatalf("reissued brief does not state why the previous result was rejected: %v", err)
					}
				} else if result.Instruction.Question == nil || result.Instruction.Question.StopCategory != "malformed_result" {
					t.Fatalf("missing human escalation: %+v", result)
				}
				records, err := e.records()
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, record := range records {
					if failure, ok := record.Payload.(*ledger.ResultRejection); ok {
						count++
						if failure.Reason == "" || failure.ResultPath == "" {
							t.Fatal("failure details missing")
						}
					}
				}
				if count != attempt+1 {
					t.Fatalf("rejected result was read again: %d failures", count)
				}
			}
			answerResult(t, e, result, ledger.AnswerText)
			result, err = e.Next(ctx)
			if err != nil || result.Instruction.Kind != form.InstructionPlan {
				t.Fatalf("human instructions did not resume planner: %+v, %v", result, err)
			}
			planResult(t, e, result)
			if _, err := e.Next(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanCheckKeepsOriginalRequestAndStartTask(t *testing.T) {
	for _, original := range []string{"Preserve storage and rename the button", ""} {
		t.Run(map[bool]string{true: "observed", false: "unobserved"}[original != ""], func(t *testing.T) {
			e, ctx := fixture(t)
			if original != "" {
				h := hook.Handler{Root: e.Root()}
				if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.UserPromptSubmit, SessionID: e.sessionID, Prompt: original}); err != nil {
					t.Fatal(err)
				}
			}
			task := "Rename the button"
			start, err := e.Start(ctx, StartRequest{Template: "test", Task: task})
			if err != nil {
				t.Fatal(err)
			}
			planResult(t, e, start)
			detection, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			acceptDetectionResult(t, e, detection.Instruction)
			review, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			before, err := e.readInstruction(review.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			if before.Request.StartTask != task || (before.Request.OriginalRequest == nil) != (original == "") || original != "" && *before.Request.OriginalRequest != original {
				t.Fatalf("plan request lost fixed inputs: %+v", before.Request)
			}
			observeAnswer(t, e, "Later input must not replace the original request")
			write(t, e.workRoot(), "inbox/task.txt", "Later task must not replace the start task")
			if err := os.RemoveAll(filepath.Join(e.workRoot(), "brief")); err != nil {
				t.Fatal(err)
			}
			body, err := e.instructionBrief(review.Instruction.ID)
			if err != nil || !strings.Contains(string(body), task) || strings.Contains(string(body), "Later task") || strings.Contains(string(body), "Later input") {
				t.Fatalf("regeneration changed plan inputs: %v", err)
			}
			var decoded check.Request
			if err := json.Unmarshal(marshal(t, before.Request), &decoded); err != nil || !reflect.DeepEqual(decoded, *before.Request) {
				t.Fatalf("fixed request inputs did not round-trip: %v", err)
			}
		})
	}
}

func TestMalformedJudgeAndDetectionResultsReissue(t *testing.T) {
	for _, kind := range []form.InstructionKind{form.InstructionPlanCheck, form.InstructionDetectionTest, form.InstructionJudgment} {
		t.Run(string(kind), func(t *testing.T) {
			e, ctx := fixture(t)
			configureStage(t, e, func(s *plan.Stage) {
				s.Checks = []plan.Check{judgeCheck("wording", false)}
			})
			start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
			if err != nil {
				t.Fatal(err)
			}
			planResult(t, e, start)
			result, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if kind != form.InstructionDetectionTest {
				acceptDetectionResult(t, e, result.Instruction)
				result, err = e.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}
			}
			if kind == form.InstructionJudgment {
				data, err := e.readInstruction(result.Instruction.ID)
				if err != nil {
					t.Fatal(err)
				}
				collect(t, e, result.Instruction, pass(*data.Request))
				work, err := e.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
					t.Fatal(err)
				}
				result, err = e.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}
			}
			before, err := e.readInstruction(result.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Instruction.Kind != kind {
				t.Fatalf("unexpected instruction: %+v", result)
			}
			collect(t, e, result.Instruction, map[string]any{"instruction_id": result.Instruction.ID})
			reissued, err := e.Next(ctx)
			if err != nil || reissued.Instruction.Kind != kind || reissued.Instruction.Reissue != 1 || reissued.Instruction.ID == result.Instruction.ID {
				t.Fatalf("malformed result did not reissue: %+v, %v", reissued, err)
			}
			after, err := e.readInstruction(reissued.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			before.Request.InstructionID = after.Request.InstructionID
			for i := range before.Request.Norms {
				before.Request.Norms[i].Content = strings.ReplaceAll(before.Request.Norms[i].Content, result.Instruction.ID+"-history.md", reissued.Instruction.ID+"-history.md")
			}
			if kind == form.InstructionDetectionTest {
				if before.Context != nil && before.Context.WorkHistory != "" || after.Context != nil && after.Context.WorkHistory != "" {
					t.Fatal("detection request received a work history")
				}
			} else if before.Context.WorkHistory != after.Context.WorkHistory {
				t.Fatal("reissued request changed fixed history contents")
			}
			if !reflect.DeepEqual(before.Request, after.Request) || before.Reference != after.Reference || !reflect.DeepEqual(before.Detection, after.Detection) {
				t.Fatal("reissued request changed fixed judgment inputs")
			}
			if kind == form.InstructionDetectionTest {
				acceptDetectionResult(t, e, reissued.Instruction)
			} else {
				collect(t, e, reissued.Instruction, pass(*after.Request))
			}
			if _, err := e.Next(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
