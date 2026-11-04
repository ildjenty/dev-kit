package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

func TestRequirementOnlyDefaultLifecycle(t *testing.T) {
	e, ctx := fixture(t)
	// The fixture initializes and commits a fresh repository. Use the shipped default policy.
	settings := "kit_version = 'v2'\n"
	write(t, e.Root(), "devkit.toml", settings)
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Use default settings")
	const original = "Create a greeting command that prints Hello and exits successfully."
	h := hook.Handler{Root: e.Root()}
	if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.UserPromptSubmit, SessionID: e.sessionID, Prompt: original}); err != nil {
		t.Fatal(err)
	}
	started, err := e.Start(ctx, StartRequest{Task: "Build a greeting command"})
	if err != nil {
		t.Fatal(err)
	}
	template, source, err := e.template(ctx, "HEAD", "default")
	if err != nil {
		t.Fatal(err)
	}
	proposal := plan.NewPlan(template, plan.ContentHash(source), "Build a greeting command", []plan.CompletionCondition{{ID: "R1", Description: "Requirements and completion conditions are agreed"}})
	bind := func(p *plan.Plan) {
		for i := range p.Stages {
			for j := range p.Stages[i].Checks {
				p.Stages[i].Checks[j].CompletionConditionIDs = []string{p.CompletionConditions[0].ID}
				p.Stages[i].Checks[j].Target = "Greeting command and repository records"
			}
		}
	}
	propose := func(instruction *form.Instruction, p plan.Plan) {
		t.Helper()
		bind(&p)
		collect(t, e, instruction, form.PlannerResult{InstructionID: instruction.ID, Plan: marshal(t, p)})
	}
	next := func() Result {
		t.Helper()
		for i := 0; i < 40; i++ {
			result, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if result.Instruction == nil {
				t.Fatalf("missing instruction: %+v", result)
			}
			switch result.Instruction.Kind {
			case form.InstructionDetectionTest:
				body, err := e.instructionBrief(result.Instruction.ID)
				if err != nil {
					t.Fatal(err)
				}
				for _, forbidden := range []string{`"expected":`, `"kind": "violation"`, `"spec":`, `"file": "violation.md"`} {
					if strings.Contains(string(body), forbidden) {
						t.Fatalf("detection leaked %s", forbidden)
					}
				}
				acceptDetectionResult(t, e, result.Instruction)
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
		t.Fatal("check instruction loop did not terminate")
		return Result{}
	}
	approve := func(result Result, category string) {
		t.Helper()
		if result.Instruction == nil || result.Instruction.Question == nil || result.Instruction.Question.StopCategory != category {
			t.Fatalf("expected %s: %+v", category, result)
		}
		observeAnswer(t, e, "Approve the recorded request and conditions.")
		if _, err := e.Answer(ctx, AnswerRequest{QuestionID: result.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
			t.Fatal(err)
		}
	}
	submit := func(result Result, stage string) {
		t.Helper()
		state, err := e.load()
		if err != nil {
			t.Fatal(err)
		}
		run, _ := state.LatestStageRun()
		if result.Instruction.Kind != form.InstructionWork || run.StageID != stage {
			t.Fatalf("expected %s work, got %+v, %+v", stage, result, run)
		}
		if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: result.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Complete " + stage}})); err != nil {
			t.Fatal(err)
		}
	}
	propose(started.Instruction, proposal)
	approval := next()
	for _, text := range []string{original, "Build a greeting command", "R1", "Requirements and completion conditions are agreed", "Adopted stages", "command:test", "Irreversible items", "External operations", "Open questions"} {
		if !strings.Contains(approval.Instruction.Question.Text, text) {
			t.Fatalf("approval omitted %q", text)
		}
	}
	approve(approval, "plan_approval")
	purpose := next()
	write(t, e.Root(), ".devkit/requirements.md", "# Requirements\n\nThe greeting command prints Hello and exits successfully.\n")
	submit(purpose, "purpose")
	replanning := next()
	if replanning.Instruction.Kind != form.InstructionPlan {
		t.Fatalf("purpose did not trigger replanning: %+v", replanning)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	if run.State != ledger.StageCompleted || run.Commit == "" {
		t.Fatal("replanning preceded stage commit")
	}
	remaining, err := proposal.SelectStages(template, []string{"scope", "structure", "contract", "steps", "implement", "retrospective"})
	if err != nil {
		t.Fatal(err)
	}
	remaining.Version = 2
	remaining.CompletionConditions = []plan.CompletionCondition{{ID: "C1", Description: "The greeting command prints Hello and exits successfully"}}
	propose(replanning.Instruction, remaining)
	approve(next(), "plan_approval")
	for _, stage := range []string{"scope", "structure", "contract", "steps"} {
		work := next()
		write(t, e.Root(), stage+".txt", "Agreed "+stage+" output\n")
		submit(work, stage)
		if stage == "scope" {
			approve(next(), "stage_confirmation")
		}
	}
	implement := next()
	write(t, e.Root(), "greet.sh", "#!/bin/sh\nprintf 'Hello\\n'\n")
	submit(implement, "implement")
	failed, err := e.Next(ctx)
	if err != nil || !strings.Contains(failed.Message, "[commands.test]") {
		t.Fatalf("missing registration correction: %+v, %v", failed, err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ = state.LatestStageRun()
	if _, open := state.OpenRound(); open {
		t.Fatal("fix round opened before registration")
	}
	rounds := state.Rounds(run.ID)
	round := rounds[len(rounds)-1]
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range records {
		if judgment, ok := record.Payload.(*ledger.Judgment); ok && judgment.RoundID == round.ID && judgment.Aspect == "command:test" {
			found = judgment.Judgment.Judgment == check.Fail && strings.Contains(judgment.Findings[0].Scenario, "[commands.test]")
		}
	}
	if !found {
		t.Fatal("unregistered command did not produce its fixable finding")
	}
	write(t, e.Root(), "test.sh", "#!/bin/sh\nset -u\noutput=$(sh greet.sh)\nstatus=$?\ntest \"$status\" -eq 0 || exit 1\ntest \"$output\" = Hello\n")
	write(t, e.Root(), "devkit.toml", settings+"\n[commands.test]\nexecutable = '/bin/sh'\nargs = ['test.sh']\ntimeout_seconds = 5\n")
	approve(next(), "meta_loop_edit")
	implement = next()
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ = state.LatestStageRun()
	if len(state.Rounds(run.ID)) != 2 {
		t.Fatal("registration did not take effect in round two")
	}
	write(t, e.Root(), "greet.sh", "#!/bin/sh\nprintf 'Hello\\n'\nexit 7\n")
	submit(implement, "implement")
	correction := next()
	if correction.Instruction.Kind != form.InstructionWork {
		t.Fatalf("failed registered test did not produce a fix round: %+v", correction)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ = state.LatestStageRun()
	if len(state.Rounds(run.ID)) != 3 {
		t.Fatal("failed registered test did not open round three")
	}
	records, err = e.records()
	if err != nil {
		t.Fatal(err)
	}
	registeredFailure := false
	for _, record := range records {
		if j, ok := record.Payload.(*ledger.Judgment); ok && j.Aspect == "command:test" && j.Command != nil && j.Judgment.Judgment == check.Fail && j.Command.ExitCode != 0 {
			registeredFailure = true
		}
	}
	if !registeredFailure {
		t.Fatal("the test did not reject a greeting with correct stdout and a failing exit status")
	}
	write(t, e.Root(), "greet.sh", "#!/bin/sh\nprintf 'Hello\\n'\nexit 0\n")
	submit(correction, "implement")
	// The registration was approved before round two, so the completion commit does not ask again.
	retrospective := next()
	if retrospective.Instruction == nil || retrospective.Instruction.Question != nil {
		t.Fatalf("approved loop edit was confirmed twice or the stage did not complete: %+v", retrospective)
	}
	body, err := e.instructionBrief(retrospective.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(e.workRoot(), "brief", retrospective.Instruction.ID+"-history.md")
	history, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), historyPath) || !strings.Contains(string(history), "Verification command `test` is not registered") || !strings.Contains(string(history), "3 rounds") || !strings.Contains(string(history), "original") {
		t.Fatal("work history omitted command failures, rounds, or human input")
	}
	submit(retrospective, "retrospective")
	completed := next()
	if completed.Status != ledger.WorkCompleted {
		t.Fatalf("work did not complete: %+v", completed)
	}
}

func TestDetectionCanBeDisabled(t *testing.T) {
	e, ctx := fixture(t)
	write(t, e.Root(), "devkit.toml", "kit_version = 'v2'\n[approval]\nplan = 'deviation'\n[detection]\nrequired = false\n")
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Disable detection tests")
	started, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, started)
	review, err := e.Next(ctx)
	if err != nil || review.Instruction == nil || review.Instruction.Kind != form.InstructionPlanCheck {
		t.Fatalf("disabled detection was issued: %+v, %v", review, err)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, review.Instruction, pass(*data.Request))
	work, err := e.Next(ctx)
	if err != nil || work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("plan did not run without a detection pass: %+v, %v", work, err)
	}
}

func TestGitSettingsAndEmptyStage(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "changed"
		if empty {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			e, ctx := fixture(t)
			write(t, e.Root(), "devkit.toml", "kit_version = 'v2'\ndefault_plan = 'test'\n[approval]\nplan = 'deviation'\n[git]\nbranch = 'topic/{work_id}/work'\ncommit_message = '{task}: {stage} ({work_id})'\nallow_empty_commit = false\n[detection]\nrequired = false\n")
			git(t, e.Root(), "add", ".")
			git(t, e.Root(), "commit", "-qm", "Configure git behavior")
			head := git(t, e.Root(), "rev-parse", "HEAD")
			started, err := e.Start(ctx, StartRequest{Task: "Update documentation"})
			if err != nil {
				t.Fatal(err)
			}
			if got := git(t, e.Root(), "branch", "--show-current"); got != "topic/"+e.workID+"/work" {
				t.Fatalf("branch setting ignored: %s", got)
			}
			planResult(t, e, started)
			review, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			data, err := e.readInstruction(review.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			collect(t, e, review.Instruction, pass(*data.Request))
			work, err := e.Next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !empty {
				write(t, e.Root(), "README.md", "# Changed\n")
			}
			if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
				t.Fatal(err)
			}
			done, err := e.Next(ctx)
			if err != nil || done.Status != ledger.WorkCompleted {
				t.Fatalf("stage did not finish: %+v, %v", done, err)
			}
			current := git(t, e.Root(), "rev-parse", "HEAD")
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			run, _ := state.LatestStageRun()
			if run.Commit != current {
				t.Fatal("completion did not record HEAD")
			}
			if empty && current != head {
				t.Fatal("empty stage created a commit")
			}
			if !empty && git(t, e.Root(), "log", "-1", "--format=%s") != "Update documentation: implement ("+e.workID+")" {
				t.Fatal("commit message setting ignored")
			}
		})
	}
}

func TestRejectedMetaLoopEditUsesConfirmedNorms(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	round, _ := state.OpenRound()
	if err := e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "Review the norm edits"}); err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), ".devkit/rules.md", "Rejected rule marker\n")
	asked, err := e.Next(ctx)
	if err != nil || asked.Instruction.Question == nil || asked.Instruction.Question.StopCategory != "meta_loop_edit" {
		t.Fatalf("norm edit did not stop: %+v, %v", asked, err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.OpenRound(); ok {
		t.Fatal("round opened before confirmation")
	}
	observeAnswer(t, e, "Reject the rule change. Undo it.")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: asked.Instruction.Question.ID, Interpretation: ledger.AnswerRejected, Text: "Undo the rule change"}); err != nil {
		t.Fatal(err)
	}
	correction, err := e.Next(ctx)
	if err != nil || correction.Instruction.Kind != form.InstructionWork {
		t.Fatalf("rejection did not return to the worker: %+v, %v", correction, err)
	}
	body, err := e.instructionBrief(correction.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Rejected rule marker") || !strings.Contains(string(body), "Undo the rejected development loop edits") {
		t.Fatal("correction did not use the confirmed norms and undo instructions")
	}
	if correction.Instruction.ID == work.Instruction.ID {
		t.Fatal("rejection reused the old work instruction")
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	fixed, _ := state.OpenRound()
	actual, err := e.workspace.SnapshotTree(ctx)
	if err != nil || fixed.StartTarget.Tree != actual || roundReference(fixed) == actual {
		t.Fatalf("rejection replaced the actual work target: %+v, %v", fixed, err)
	}
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: correction.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	again, err := e.Next(ctx)
	if err != nil || again.Instruction == nil || again.Instruction.Kind != form.InstructionWork {
		t.Fatalf("unchanged rejected norms were asked again: %+v, %v", again, err)
	}
	original, err := e.workspace.ReadFile(ctx, roundReference(fixed), ".devkit/rules.md")
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), ".devkit/rules.md", string(original))
	write(t, e.Root(), "README.md", "# Preserved work product\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: again.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	done, err := e.Next(ctx)
	if err != nil || done.Status != ledger.WorkCompleted {
		t.Fatalf("undo did not complete: %+v, %v", done, err)
	}
	if git(t, e.Root(), "show", "HEAD:README.md") != "# Preserved work product" {
		t.Fatal("undo discarded the work product")
	}

}

func TestStartRequiresCommittedInitialization(t *testing.T) {
	for _, pending := range []string{"HEAD", "devkit.toml", ".devkit/plans/test.json", ".devkit/plans/untracked.json", ".devkit/rules.md", ".devkit/checks/plan.md", ".devkit/templates/brief/work.md.tmpl", ".devkit/principles/harness.md", ".devkit/stages/implement.md", ".claude/agents/devkit-judge.md", ".codex/agents/devkit-judge.toml", "AGENTS.md", "CLAUDE.md"} {
		t.Run(pending, func(t *testing.T) {
			e, ctx := fixture(t)
			if pending == "HEAD" {
				git(t, e.Root(), "checkout", "--orphan", "empty")
			} else {
				write(t, e.Root(), pending, "Uncommitted initialization change\n")
			}
			branch := git(t, e.Root(), "branch", "--show-current")
			if _, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"}); err == nil || !strings.Contains(err.Error(), "commit the devkit init changes first") {
				t.Fatalf("start did not explain initialization requirement: %v", err)
			}
			if e.workID != "" || git(t, e.Root(), "branch", "--show-current") != branch {
				t.Fatal("failed start created work or switched branches")
			}
			works, err := os.ReadDir(filepath.Join(e.runRoot(), "work"))
			if err != nil || len(works) != 0 {
				t.Fatalf("failed start wrote work records: %v, %v", works, err)
			}
		})
	}
}
