package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/external"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

func configureStage(t *testing.T, e *Engine, configure func(*plan.Stage)) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	configure(&template.Stages[0])
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure stage")
}

func TestGateIgnoresApprovalOfAnotherQuestion(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) { stage.Gate.Required = true })
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Gate target\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	q, err := e.saveQuestion(state, e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: round.ReturnedTarget.Tree}, "scope_confirmation", "Approve the scope?", "implement"), "")
	if err != nil {
		t.Fatal(err)
	}
	observeAnswer(t, e, "Approve the scope.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	asked, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if asked.Status != ledger.WorkActive || asked.Instruction == nil || asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != "stage_confirmation" {
		t.Fatalf("unrelated approval bypassed the gate: %+v", asked)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	observeAnswer(t, e, "Approve this gate.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	if !gateApproved(state, run, round, q.ApprovalTarget) {
		t.Fatal("current gate approval was not effective")
	}
	otherRun := *run
	otherRun.ID = "another-run"
	otherRound := *round
	otherRound.ID = "another-round"
	if gateApproved(state, &otherRun, round, q.ApprovalTarget) || gateApproved(state, run, &otherRound, q.ApprovalTarget) {
		t.Fatal("gate approval escaped its stage run or round")
	}
}

func TestSubmissionRecovery(t *testing.T) {
	for _, retry := range []string{"submit", "next"} {
		for _, stop := range []string{"progress", "returned_target"} {
			t.Run(retry+"_after_"+stop, func(t *testing.T) {
				e, ctx := fixture(t)
				configureStage(t, e, func(stage *plan.Stage) { stage.Gate.Required = true })
				work := working(t, e, ctx)
				write(t, e.Root(), "README.md", "# Saved submission\n")
				progress := form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Update documentation"}}
				if _, err := e.Submit(ctx, marshal(t, progress)); err != nil {
					t.Fatal(err)
				}
				records, err := e.records()
				if err != nil {
					t.Fatal(err)
				}
				cutoff := ""
				for _, record := range records {
					if _, ok := record.Payload.(*ledger.Progress); ok && stop == "progress" {
						cutoff = record.ID
					}
					if round, ok := record.Payload.(*ledger.Round); ok && round.ReturnedTarget != nil && stop == "returned_target" {
						cutoff = record.ID
					}
				}
				if cutoff == "" {
					t.Fatal("submission interruption point was not found")
				}
				for _, record := range records {
					if record.ID > cutoff {
						if err := os.Remove(filepath.Join(e.workRoot(), "records", record.ID+".json")); err != nil {
							t.Fatal(err)
						}
					}
				}
				// Recovery must use the saved submission even when the checkout changes.
				write(t, e.Root(), "README.md", "# Later checkout\n")
				reopened, err := newTestEngine(ctx, e.Root(), e.sessionID)
				if err != nil {
					t.Fatal(err)
				}
				var asked Result
				if retry == "submit" {
					if _, err := reopened.Submit(ctx, marshal(t, progress)); err != nil {
						t.Fatal(err)
					}
				} else {
					asked, err = reopened.Next(ctx)
					if err != nil || asked.Instruction == nil || asked.Instruction.Question == nil {
						t.Fatalf("next did not recover the submission: %+v, %v", asked, err)
					}
				}
				state, err := reopened.load()
				if err != nil {
					t.Fatal(err)
				}
				round, _ := state.OpenRound()
				saved, err := reopened.workspace.ReadFile(ctx, round.ReturnedTarget.Tree, "README.md")
				if err != nil || string(saved) != "# Saved submission\n" {
					t.Fatalf("recovery replaced the submitted target: %q, %v", saved, err)
				}
				write(t, e.Root(), "README.md", "# Saved submission\n")
				if asked.Instruction == nil {
					asked, err = reopened.Next(ctx)
					if err != nil || asked.Instruction == nil || asked.Instruction.Question == nil {
						t.Fatalf("expected completion gate: %+v, %v", asked, err)
					}
				}
				observeAnswer(t, reopened, "Approve the recovered submission.")
				if _, err := reopened.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
					t.Fatal(err)
				}
				result, err := reopened.Next(ctx)
				if err != nil || result.Status != ledger.WorkCompleted {
					t.Fatalf("recovered submission did not complete: %+v, %v", result, err)
				}
				records, err = reopened.records()
				if err != nil {
					t.Fatal(err)
				}
				progressCount, artifacts := 0, 0
				for _, record := range records {
					if _, ok := record.Payload.(*ledger.Progress); ok {
						progressCount++
					}
					if artifact, ok := record.Payload.(*ledger.Artifact); ok && artifact.ChangeKind == "submission" {
						artifacts++
					}
				}
				if progressCount != 1 || artifacts != 1 {
					t.Fatalf("recovery duplicated submission records: %d, %d", progressCount, artifacts)
				}
			})
		}
	}
}

func TestCommitRecoveryAfterPauseInHook(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Commit once\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	pause, err := ledger.NewRecord(e.store(), &ledger.Pause{Position: form.Reference{RecordID: state.LastRecordID()}, Reason: "Pause during commit"})
	if err != nil {
		t.Fatal(err)
	}
	pauseData := fsstore.Record{ID: pause.ID, Kind: string(ledger.KindPause), Payload: marshal(t, pause.Payload)}
	write(t, e.workRoot(), "pause.json", string(marshal(t, pauseData)))
	relative, err := filepath.Rel(e.Root(), e.workRoot())
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), ".git/hooks/pre-commit", "#!/bin/sh\nprintf 'hook ran\\n' >> '"+relative+"/hook-log'\ncp '"+relative+"/pause.json' '"+relative+"/records/"+pause.ID+".json'\n")
	if err := os.Chmod(filepath.Join(e.Root(), ".git/hooks/pre-commit"), 0755); err != nil {
		t.Fatal(err)
	}
	paused, err := e.Next(ctx)
	if err != nil || !paused.Paused {
		t.Fatalf("commit did not pause before completion records: %+v, %v", paused, err)
	}
	head := git(t, e.Root(), "rev-parse", "HEAD")
	reopened, err := newTestEngine(ctx, e.Root(), e.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := reopened.Next(ctx)
	if err != nil || result.Status != ledger.WorkCompleted {
		t.Fatalf("commit recovery failed: %+v, %v", result, err)
	}
	if actual := git(t, e.Root(), "rev-parse", "HEAD"); actual != head {
		t.Fatalf("recovery made another commit: %s != %s", actual, head)
	}
	log, err := os.ReadFile(filepath.Join(e.workRoot(), "hook-log"))
	if err != nil || string(log) != "hook ran\n" {
		t.Fatalf("pre-commit ran more than once: %q, %v", log, err)
	}
}

func TestPauseAndStatusWithoutForwardAssets(t *testing.T) {
	for _, broken := range []string{"devkit.toml"} {
		t.Run(filepath.Base(broken), func(t *testing.T) {
			e, ctx := fixture(t)
			working(t, e, ctx)
			write(t, e.Root(), broken, "Invalid configuration or schema")
			git(t, e.Root(), "add", broken)
			git(t, e.Root(), "commit", "-qm", "Commit invalid forward configuration")
			reopened, err := newTestEngine(ctx, e.Root(), e.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Status(ctx); err != nil {
				t.Fatal(err)
			}
			if paused, err := reopened.Pause(ctx, "Repair configuration"); err != nil || !paused.Paused {
				t.Fatalf("pause depended on forward assets: %+v, %v", paused, err)
			}
			if result, err := reopened.Next(ctx); err != nil || !result.Paused || result.Instruction != nil {
				t.Fatalf("paused next depended on forward assets: %+v, %v", result, err)
			}
			if _, err := reopened.Resume(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Next(ctx); err == nil {
				t.Fatal("forward operation ignored invalid assets")
			}
		})
	}
}

func TestExpansionRequiresCompletionMarker(t *testing.T) {
	e, ctx := fixture(t)
	working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	path, err := e.expanded(ctx, round.StartTarget.Tree)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path + ".complete"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "README.md")
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	write(t, path, "README.md", "Incomplete expansion")
	if _, err := e.expanded(ctx, round.StartTarget.Tree); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(file)
	if err != nil || string(content) != "# Test repository\n" {
		t.Fatalf("incomplete expansion was reused: %q, %v", content, err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0444 {
		t.Fatalf("expansion is not read-only: %v, %v", info, err)
	}
	if _, err := os.Stat(path + ".complete"); err != nil {
		t.Fatal(err)
	}
}

func TestMissingBriefIsRegenerated(t *testing.T) {
	for _, kind := range []string{"planner", "work"} {
		t.Run(kind, func(t *testing.T) {
			e, ctx := fixture(t)
			var issued Result
			var err error
			if kind == "planner" {
				issued, err = e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
			} else {
				issued = working(t, e, ctx)
			}
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(issued.Instruction.Brief)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(e.workRoot(), "brief")); err != nil {
				t.Fatal(err)
			}
			write(t, e.Root(), "README.md", "# Later checkout\n")
			result, err := e.Next(ctx)
			if err != nil || result.Instruction == nil || result.Instruction.ID != issued.Instruction.ID {
				t.Fatalf("brief regeneration changed the instruction: %+v, %v", result, err)
			}
			content, err := os.ReadFile(result.Instruction.Brief)
			if err != nil || string(content) != string(original) {
				t.Fatalf("brief did not use the original reference: %v", err)
			}
		})
	}
}

func TestEndedWorkAcceptsAnswerCorrections(t *testing.T) {
	for _, ended := range []ledger.WorkStatus{ledger.WorkCompleted, ledger.WorkAborted} {
		t.Run(string(ended), func(t *testing.T) {
			e, ctx := fixture(t)
			work := working(t, e, ctx)
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			q, err := e.saveQuestion(state, e.question(form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: e.workID}, "delegation", "Approve the documentation scope?", "implement"), "")
			if err != nil {
				t.Fatal(err)
			}
			observeAnswer(t, e, "Approve this scope.")
			if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerApproved}); err != nil {
				t.Fatal(err)
			}
			state, err = e.load()
			if err != nil {
				t.Fatal(err)
			}
			previous := state.AnsweredQuestions()[q.ID]
			if ended == ledger.WorkCompleted {
				if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
					t.Fatal(err)
				}
				if result, err := e.Next(ctx); err != nil || result.Status != ended {
					t.Fatalf("work did not end: %+v, %v", result, err)
				}
			} else if _, err := e.Abort(ctx); err != nil {
				t.Fatal(err)
			}
			observeAnswer(t, e, "Withdraw the earlier approval.")
			if result, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerRejected, Text: "Withdraw the scope approval", Supersedes: previous.RecordID}); err != nil || result.Status != ended {
				t.Fatalf("ended work rejected correction: %+v, %v", result, err)
			}
			state, err = e.load()
			if err != nil {
				t.Fatal(err)
			}
			if answer := state.AnsweredQuestions()[q.ID]; answer.Answer.Interpretation != ledger.AnswerRejected || answer.Answer.Supersedes != previous.RecordID {
				t.Fatalf("correction was not effective: %+v", answer)
			}
			if _, err := e.append(&ledger.Question{Question: e.question(form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: e.workID}, "followup", "Approve the follow-up scope?", "implement")}); err != nil {
				t.Fatal(err)
			}
			state, err = e.load()
			if err != nil {
				t.Fatal(err)
			}
			questions := state.UnansweredQuestions()
			observeAnswer(t, e, "Approve the follow-up scope.")
			if _, err := e.Answer(ctx, AnswerRequest{QuestionID: questions[0].ID, Interpretation: ledger.AnswerApproved}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAuthorizationsIncludeDecisionsAndDelegation(t *testing.T) {
	e, ctx := fixture(t)
	working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	targets := []form.ApprovalTarget{{Kind: form.ApprovalDecision, DecisionID: "scope-decision"}, {Kind: form.ApprovalWork, WorkID: e.workID}}
	for i, target := range targets {
		q, err := e.saveQuestion(state, e.question(target, "scope", "Define the delegated scope", "implement"), "")
		if err != nil {
			t.Fatal(err)
		}
		interpretation := ledger.AnswerApproved
		if i == 1 {
			interpretation = ledger.AnswerText
		}
		// These records model already accepted human decisions without advancing the loop.
		answer := ledger.Answer{QuestionID: q.ID, Target: target, Interpretation: interpretation, Text: "Update documentation within this scope", Original: "I delegate documentation edits", SessionID: e.sessionID, ObservedAt: time.Now().UTC()}
		if interpretation == ledger.AnswerApproved {
			_, err = e.append(&ledger.Approval{Answer: answer})
		} else {
			_, err = e.append(&answer)
		}
		if err != nil {
			t.Fatal(err)
		}
		state, err = e.load()
		if err != nil {
			t.Fatal(err)
		}
	}
	round, _ := state.OpenRound()
	_, data, _, err := e.judgmentInstruction(ctx, state, catalogJudge("plan", false), form.InstructionPlanCheck, "authorization-test", round.StartTarget.Tree, form.TargetVersion{Kind: form.TargetPlan, Plan: 1}, e.Root(), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	// The approval of the first plan is an authorization of plan 1 as well.
	if len(data.Request.Authorizations) != len(targets)+1 {
		t.Fatalf("missing authorization inputs: %+v", data.Request.Authorizations)
	}
	for _, authorization := range data.Request.Authorizations {
		if authorization.Target.Kind == form.ApprovalPlan {
			continue
		}
		if len(authorization.Evidence) != 1 || authorization.Evidence[0].RecordID == "" || !strings.Contains(authorization.Scope, "Update documentation within this scope") || !strings.Contains(authorization.Scope, "I delegate documentation edits") {
			t.Fatalf("authorization lost its scope or provenance: %+v", authorization)
		}
	}
}

func TestPlannerOmitsCorrectedHumanInstructions(t *testing.T) {
	e, ctx := fixture(t)
	working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	q, err := e.saveQuestion(state, e.question(form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: e.workID}, "scope", "Set the scope", "implement"), "")
	if err != nil {
		t.Fatal(err)
	}
	observeAnswer(t, e, "Follow the withdrawn scope.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Withdrawn instruction marker"}); err != nil {
		t.Fatal(err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	previous := state.AnsweredQuestions()[q.ID]
	observeAnswer(t, e, "Follow the current scope.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Current instruction marker", Supersedes: previous.RecordID}); err != nil {
		t.Fatal(err)
	}
	result, err := e.planner(ctx, "test", 2, plan.HumanProposal, "")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result.Instruction.Brief)
	if err != nil || strings.Contains(string(body), "Withdrawn instruction marker") || !strings.Contains(string(body), "Current instruction marker") {
		t.Fatalf("planner received superseded human instructions: %v", err)
	}
}

func TestPreCommitPathFollowsGitConfiguration(t *testing.T) {
	e, ctx := fixture(t)
	git(t, e.Root(), "config", "core.hooksPath", ".custom-hooks")
	path, err := e.workspace.PreCommitPath(ctx)
	if err != nil || path != ".custom-hooks/pre-commit" {
		t.Fatalf("effective hook path was not resolved: %q, %v", path, err)
	}
}

func TestSuccessfulVerificationCommand(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[commands.verify]\nexecutable = '/bin/sh'\nargs = ['-c', \"printf 'Verified target\\\\n'\"]\ntimeout_seconds = 5\n")
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:verify", Required: true}}
	})
	work := working(t, e, ctx)
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	if result, err := e.Next(ctx); err != nil || result.Status != ledger.WorkCompleted {
		t.Fatalf("successful command did not complete: %+v, %v", result, err)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if judgment, ok := record.Payload.(*ledger.Judgment); ok && judgment.RoundID != "" {
			if judgment.Judgment.Judgment != check.Pass || judgment.Command == nil || judgment.Command.ExitCode != 0 || judgment.Command.Output != "Verified target\n" {
				t.Fatalf("missing successful command evidence: %+v", judgment)
			}
			return
		}
	}
	t.Fatal("successful command result was not recorded")
}

func TestRoundOpeningConfirmsUncommittedAspect(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("plan", true)}
	})
	working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	round, _ := state.OpenRound()
	if err := e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "Revise the aspect before the next round"}); err != nil {
		t.Fatal(err)
	}
	path := ".devkit/principles/review/plan.md"
	aspect, err := os.ReadFile(filepath.Join(e.Root(), path))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), path, string(aspect)+"\nUpdated criteria.\n")
	result, err := e.Next(ctx)
	if err != nil || result.Instruction == nil || result.Instruction.Question == nil || result.Instruction.Question.StopCategory != config.MetaLoopEdit {
		t.Fatalf("uncommitted aspect did not require confirmation: %+v, %v", result, err)
	}
	observeAnswer(t, e, "Approve the updated aspect.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: result.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	opened, err := e.Next(ctx)
	if err != nil || opened.Instruction == nil || opened.Instruction.Kind != form.InstructionWork {
		t.Fatalf("approved opening norms did not open the round: %+v, %v", opened, err)
	}
}

func TestConfiguredPreCommitEditRequiresConfirmation(t *testing.T) {
	e, ctx := fixture(t)
	git(t, e.Root(), "config", "core.hooksPath", ".custom-hooks")
	work := working(t, e, ctx)
	write(t, e.Root(), ".custom-hooks/pre-commit", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(e.Root(), ".custom-hooks/pre-commit"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	result, err := e.Next(ctx)
	if err != nil || result.Status != ledger.WorkActive || result.Instruction == nil || result.Instruction.Question == nil {
		t.Fatalf("configured pre-commit edit bypassed confirmation: %+v, %v", result, err)
	}
}

func TestHistoryReadErrorsArePropagated(t *testing.T) {
	e, ctx := fixture(t)
	working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	round, _ := state.OpenRound()
	write(t, e.workRoot(), "records/"+state.LastRecordID()+".json", "Corrupt record")
	if _, err := e.outputs(state); err == nil {
		t.Fatal("outputs hid a record read error")
	}
	if _, _, err := e.boundPlan(1); err == nil {
		t.Fatal("bound plan hid a record read error")
	}
	if _, err := e.planRounds(); err == nil {
		t.Fatal("plan round count hid a record read error")
	}
	if _, err := e.roundWindow(state, run.ID, round); err == nil {
		t.Fatal("round window hid a record read error")
	}
}

func TestBriefOutputsPreserveDecisionConditions(t *testing.T) {
	e, ctx := fixture(t)
	working(t, e, ctx)
	decision := form.Decision{
		ID: "conditional", Content: "Keep the current service", Validity: "While the owner accepts the cost",
		Conditions: []string{"The requirements still apply"}, Decider: "human",
		Supersedes: []form.Reference{}, DependsOn: []form.Reference{{External: "https://example.invalid/service"}},
		RejectedAlternatives: []form.RejectedAlternative{}, Evidence: []form.Reference{},
	}
	if _, err := e.Note(ctx, ledger.KindDecision, marshal(t, decision)); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := e.outputs(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range outputs {
		if output.StageID != "Decision "+decision.ID {
			continue
		}
		var got form.Decision
		if err := json.Unmarshal([]byte(output.Content), &got); err != nil {
			t.Fatal(err)
		}
		if string(marshal(t, got)) != string(marshal(t, decision)) {
			t.Fatalf("brief output changed decision conditions: %+v", got)
		}
		return
	}
	t.Fatal("brief outputs omitted the conditional decision")
}

func TestCommitRecoveryCompletesPartialRecords(t *testing.T) {
	for _, stop := range []string{"commit_artifact", "closed_round", "completed_stage"} {
		t.Run(stop, func(t *testing.T) {
			e, ctx := fixture(t)
			work := working(t, e, ctx)
			write(t, e.Root(), "README.md", "# Persisted completion\n")
			if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
				t.Fatal(err)
			}
			if result, err := e.Next(ctx); err != nil || result.Status != ledger.WorkCompleted {
				t.Fatalf("initial completion failed: %+v, %v", result, err)
			}
			head := git(t, e.Root(), "rev-parse", "HEAD")
			records, err := e.records()
			if err != nil {
				t.Fatal(err)
			}
			cutoff := ""
			for _, record := range records {
				switch value := record.Payload.(type) {
				case *ledger.Artifact:
					if stop == "commit_artifact" && value.ChangeKind == "commit" {
						cutoff = record.ID
					}
				case *ledger.Round:
					if stop == "closed_round" && value.Decision != nil && value.Decision.Action == plan.Done {
						cutoff = record.ID
					}
				case *ledger.StageRun:
					if stop == "completed_stage" && value.State == ledger.StageCompleted {
						cutoff = record.ID
					}
				}
			}
			if cutoff == "" {
				t.Fatal("completion interruption point was not found")
			}
			for _, record := range records {
				if record.ID > cutoff {
					if err := os.Remove(filepath.Join(e.workRoot(), "records", record.ID+".json")); err != nil {
						t.Fatal(err)
					}
				}
			}
			reopened, err := newTestEngine(ctx, e.Root(), e.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := reopened.Next(ctx); err != nil || result.Status != ledger.WorkCompleted {
				t.Fatalf("partial completion was not recovered: %+v, %v", result, err)
			}
			if actual := git(t, e.Root(), "rev-parse", "HEAD"); actual != head {
				t.Fatal("partial completion made a duplicate commit")
			}
		})
	}
}

func TestCommitRefreshWaitsForJudgeAndApproval(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Gate.Required = true
		stage.Checks = []plan.Check{judgeCheck("plan", true)}
	})
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Submitted target\n")
	write(t, e.Root(), ".git/hooks/pre-commit", "#!/bin/sh\nprintf '# Formatted target\\n' > README.md\ngit add README.md\n")
	if err := os.Chmod(filepath.Join(e.Root(), ".git/hooks/pre-commit"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"submitted", "refreshed"} {
		review, err := e.Next(ctx)
		if err != nil || review.Status != ledger.WorkActive || review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
			t.Fatalf("%s target did not await judgment: %+v, %v", phase, review, err)
		}
		pending, err := e.Next(ctx)
		if err != nil || pending.Instruction == nil || pending.Instruction.ID != review.Instruction.ID || pending.Status != ledger.WorkActive {
			t.Fatalf("%s target bypassed pending judgment: %+v, %v", phase, pending, err)
		}
		data, err := e.readInstruction(review.Instruction.ID)
		if err != nil {
			t.Fatal(err)
		}
		collect(t, e, review.Instruction, pass(*data.Request))
		asked, err := e.Next(ctx)
		if err != nil || asked.Status != ledger.WorkActive || asked.Instruction == nil || asked.Instruction.Question == nil {
			t.Fatalf("%s target did not await approval: %+v, %v", phase, asked, err)
		}
		if pending, err := e.Next(ctx); err != nil || pending.Status != ledger.WorkActive || pending.Instruction == nil || pending.Instruction.ID != asked.Instruction.ID {
			t.Fatalf("%s target bypassed pending approval: %+v, %v", phase, pending, err)
		}
		observeAnswer(t, e, "Approve this target.")
		if _, err := e.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
			t.Fatal(err)
		}
	}
	if done, err := e.Next(ctx); err != nil || done.Status != ledger.WorkCompleted {
		t.Fatalf("approved target did not complete: %+v, %v", done, err)
	}
}

func TestVerificationRunsAgainInNewRound(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[commands.verify]\nexecutable = '/bin/sh'\nargs = ['-c', \"printf 'attempt\\\\n' >> .devkit/run/attempts; test -f .devkit/run/retry\"]\ntimeout_seconds = 5\n")
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:verify", Required: true}}
	})
	work := working(t, e, ctx)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
			t.Fatal(err)
		}
		result, err := e.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			if result.Instruction == nil || result.Instruction.Kind != form.InstructionWork {
				t.Fatalf("failed command did not request correction: %+v", result)
			}
			write(t, e.Root(), ".devkit/run/retry", "Ready\n")
			work = result
		} else if result.Status != ledger.WorkCompleted {
			t.Fatalf("previous failure was reused: %+v", result)
		}
	}
	attempts, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/run/attempts"))
	if err != nil || strings.Count(string(attempts), "attempt\n") != 2 {
		t.Fatalf("expected one command per round: %q, %v", attempts, err)
	}
}

func TestOptionalCommandMutationStopsBeforeJudge(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[commands.verify]\nexecutable = '/bin/sh'\nargs = ['-c', \"printf 'Changed target\\\\n' > README.md\"]\ntimeout_seconds = 5\n")
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:verify"}, judgeCheck("plan", false)}
	})
	work := working(t, e, ctx)
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	result, err := e.Next(ctx)
	if err != nil || result.Instruction == nil || result.Instruction.Kind != form.InstructionWork {
		t.Fatalf("optional mutation proceeded to judgment: %+v, %v", result, err)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if judgment, ok := record.Payload.(*ledger.Judgment); ok && judgment.RoundID != "" {
			if !commandTargetChanged(judgment.Judgment) || judgment.Command == nil || judgment.Command.ExitCode != 0 || judgment.Command.TimedOut {
				t.Fatalf("actual command result was not preserved: %+v", judgment)
			}
			return
		}
	}
	t.Fatal("target mutation was not recorded")
}

func TestUnrelatedApprovalDoesNotClearChangeConfirmation(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	write(t, e.Root(), "package.json", "{\"name\":\"fixture\"}\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	q, err := e.saveQuestion(state, e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: round.ReturnedTarget.Tree}, "scope", "Approve the documentation scope?", "implement"), "")
	if err != nil {
		t.Fatal(err)
	}
	observeAnswer(t, e, "Approve the documentation scope.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	asked, err := e.Next(ctx)
	if err != nil || asked.Status != ledger.WorkActive || asked.Instruction == nil || asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != "stage_confirmation" {
		t.Fatalf("unrelated approval cleared dependency confirmation: %+v, %v", asked, err)
	}
	observeAnswer(t, e, "Approve the dependency change.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	if done, err := e.Next(ctx); err != nil || done.Status != ledger.WorkCompleted {
		t.Fatalf("change confirmation did not permit completion: %+v, %v", done, err)
	}
}

func TestHumanStageUsesItsOwnAnswer(t *testing.T) {
	e, ctx := fixture(t)
	configureStage(t, e, func(stage *plan.Stage) { stage.Assignee = plan.Human })
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
	asked, err := e.Next(ctx)
	asked = approvedNext(t, e, asked)
	if err != nil || asked.Instruction == nil || asked.Instruction.Question == nil {
		t.Fatalf("human stage did not ask for output: %+v, %v", asked, err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	round, _ := state.OpenRound()
	if run.State != ledger.StageAnswerWait || run.NextActor != "human" || run.StopReason != asked.Instruction.Question.StopReason {
		t.Fatalf("human wait was not recorded: %+v", run)
	}
	other, err := e.saveQuestion(state, e.question(asked.Instruction.Question.ApprovalTarget, "scope", "Approve a separate scope?", run.StageID), "")
	if err != nil {
		t.Fatal(err)
	}
	observeAnswer(t, e, "Approve the separate scope.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: other.ID, Interpretation: ledger.AnswerApproved, Text: "Unrelated output"}); err != nil {
		t.Fatal(err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := humanStageAnswer(state, run, round); ok {
		t.Fatal("unrelated answer became human stage output")
	}
	observeAnswer(t, e, "The reviewed documentation is ready.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerText, Text: "Human stage output"}); err != nil {
		t.Fatal(err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	otherRound := *round
	otherRound.ID = "later-round"
	otherRun := *run
	otherRun.ID = "later-run"
	if _, ok := humanStageAnswer(state, &otherRun, round); ok {
		t.Fatal("answer escaped its stage run")
	}
	if _, ok := humanStageAnswer(state, run, &otherRound); ok {
		t.Fatal("answer escaped its round")
	}
	if done, err := e.Next(ctx); err != nil || done.Status != ledger.WorkCompleted {
		t.Fatalf("human output did not complete: %+v, %v", done, err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	progress, _ := state.LatestProgress()
	if progress == nil || len(progress.WorkPerformed) != 1 || progress.WorkPerformed[0] != "Human stage output" {
		t.Fatalf("incorrect human output: %+v", progress)
	}
}

func TestDecisionAnswerRetryRestoresStateChange(t *testing.T) {
	for _, interpretation := range []ledger.Interpretation{ledger.AnswerApproved, ledger.AnswerRejected, ledger.AnswerDeferred} {
		for _, correction := range []bool{false, true} {
			name := string(interpretation)
			if correction {
				name += "_correction"
			}
			t.Run(name, func(t *testing.T) {
				e, ctx := fixture(t)
				working(t, e, ctx)
				decision := form.Decision{ID: "choice", Content: "Retain the service", Validity: "While the requirements apply", Conditions: []string{}, Decider: "human", Supersedes: []form.Reference{}, DependsOn: []form.Reference{}, RejectedAlternatives: []form.RejectedAlternative{}, Evidence: []form.Reference{}}
				if _, err := e.Note(ctx, ledger.KindDecision, marshal(t, decision)); err != nil {
					t.Fatal(err)
				}
				state, err := e.load()
				if err != nil {
					t.Fatal(err)
				}
				q, err := e.saveQuestion(state, e.question(form.ApprovalTarget{Kind: form.ApprovalDecision, DecisionID: decision.ID}, "decision", "Confirm the service decision", "implement"), "")
				if err != nil {
					t.Fatal(err)
				}
				observeAnswer(t, e, "Record this decision.")
				request := AnswerRequest{QuestionID: q.ID, Interpretation: interpretation, Text: "Service decision"}
				if correction {
					if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Pending interpretation"}); err != nil {
						t.Fatal(err)
					}
					state, err = e.load()
					if err != nil {
						t.Fatal(err)
					}
					request.Supersedes = state.AnsweredQuestions()[q.ID].RecordID
					observeAnswer(t, e, "Correct the earlier decision.")
				}
				if _, err := e.Answer(ctx, request); err != nil {
					t.Fatal(err)
				}
				state, err = e.load()
				if err != nil {
					t.Fatal(err)
				}
				answer := state.AnsweredQuestions()[q.ID]
				records, err := e.records()
				if err != nil {
					t.Fatal(err)
				}
				for _, record := range records {
					if record.ID > answer.RecordID {
						if err := os.Remove(filepath.Join(e.workRoot(), "records", record.ID+".json")); err != nil {
							t.Fatal(err)
						}
					}
				}
				for retry := 0; retry < 2; retry++ {
					if _, err := e.Answer(ctx, request); err != nil {
						t.Fatal(err)
					}
				}
				records, err = e.records()
				if err != nil {
					t.Fatal(err)
				}
				changes := 0
				for _, record := range records {
					if change, ok := record.Payload.(*ledger.StateChange); ok && change.AnswerID == answer.RecordID {
						changes++
						if change.DecisionStatus != ledger.DecisionStatus(interpretation) {
							t.Fatalf("incorrect recovered decision status: %+v", change)
						}
					}
				}
				if changes != 1 {
					t.Fatalf("expected one restored state change, got %d", changes)
				}
			})
		}
	}
}

func TestStartRefusesUnfinishedWorkBeforeInitialInstruction(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_binding", true: "after_binding"}[bound], func(t *testing.T) {
			e, ctx := fixture(t)
			request := StartRequest{Pattern: "test", Task: "Update documentation"}
			initial, err := e.Start(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			records, err := e.records()
			if err != nil {
				t.Fatal(err)
			}
			for _, record := range records {
				if _, ok := record.Payload.(*ledger.Instruction); ok {
					if err := os.Remove(filepath.Join(e.workRoot(), "records", record.ID+".json")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !bound {
				entries, err := os.ReadDir(filepath.Join(e.runRoot(), "sessions", e.sessionID))
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), "-bind.json") {
						if err := os.Remove(filepath.Join(e.runRoot(), "sessions", e.sessionID, entry.Name())); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			reopened, err := newTestEngine(ctx, e.Root(), e.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Start(ctx, request); err == nil || !strings.Contains(err.Error(), initial.WorkID) {
				t.Fatalf("interrupted start did not refuse unfinished work: %v", err)
			}
		})
	}
}

func TestTargetMismatchDoesNotInventCommandResult(t *testing.T) {
	e, ctx := fixture(t)
	working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	write(t, e.Root(), "README.md", "# Mismatching target\n")
	request := check.Request{RequestInput: check.RequestInput{InstructionID: "mismatch", Runner: "command:test", Aspect: check.Aspect{ID: "command:test"}, TargetVersion: round.StartTarget, TargetPath: e.Root()}}
	command := config.Command{Executable: "/bin/sh", Args: []string{"-c", "touch .devkit/run/unexpected-execution"}, TimeoutSeconds: 5}
	verification, err := e.runCheck(ctx, request, command, "implement", round.StartTarget.Tree)
	if err != nil {
		t.Fatal(err)
	}
	judgment, err := e.runJudge(ctx, instructionData{Request: &request}, config.Judge{Command: command}, "implement", round.StartTarget.Tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []check.Judgment{verification, judgment} {
		if !commandTargetChanged(result) || result.Command != nil {
			t.Fatalf("unexecuted command received a result: %+v", result)
		}
	}
	if _, err := os.Stat(filepath.Join(e.Root(), ".devkit/run/unexpected-execution")); !os.IsNotExist(err) {
		t.Fatalf("command ran against a mismatching target: %v", err)
	}
}

func TestExternalQuestionRecordsWaitingStage(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "publish", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(e.Root(), "publish"), 0755); err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[external.publish]\nexecutable = './publish'\nargs = []\ntimeout_seconds = 5\n")
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Register publication")
	work := working(t, e, ctx)
	asked, err := e.External(ctx, external.Request{Kind: "publish", Target: "fixture/item", Input: json.RawMessage(`{"message":"Hello"}`)})
	if err != nil || asked.Instruction == nil || asked.Instruction.Question == nil {
		t.Fatalf("external approval was not requested: %+v, %v", asked, err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	if run.State != ledger.StageExternalWait || run.NextActor != "human" || run.StopReason != asked.Instruction.Question.StopReason {
		t.Fatalf("external wait was not recorded: %+v", run)
	}
	observeAnswer(t, e, "Approve this publication.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	resumed, err := e.Next(ctx)
	if err != nil || resumed.Instruction == nil || resumed.Instruction.ID != work.Instruction.ID {
		t.Fatalf("external wait did not return to work: %+v, %v", resumed, err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ = state.LatestStageRun()
	if run.State != ledger.StageWorking || run.NextActor != "worker" {
		t.Fatalf("external wait remained after resumption: %+v", run)
	}
}
