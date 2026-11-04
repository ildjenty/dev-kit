package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

func roleWork(t *testing.T, change func(*plan.Stage)) (*Engine, Result) {
	t.Helper()
	e, ctx := fixture(t)
	configure(t, e, "[detection]\nrequired=false\n[roles.senior]\nkind='work'\nagent=''\n[commands.fail]\nexecutable='/usr/bin/false'\ntimeout_seconds=5\n")
	configureStage(t, e, change)
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	return e, roleNext(t, e)
}

func TestFixRuleCheckAndFailureCountKeepStageRunAndDiff(t *testing.T) {
	e, work := roleWork(t, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:fail", Runner: "command:fail", Required: true}}
		stage.Limits = plan.Limits{MaxRounds: 5, MaxNoProgress: 5}
		stage.Transitions.FixRules = []plan.FixRule{{When: &plan.FixCondition{Check: "command:fail", FailuresAtLeast: 2}, Assignee: "senior"}}
	})
	state, _ := e.load()
	first, _ := state.OpenRound()
	for _, name := range []string{"first.txt", "second.txt"} {
		write(t, e.Root(), name, "A cumulative change.\n")
		submitWork(t, e, work)
		if _, err := e.Next(t.Context()); err != nil {
			t.Fatal(err)
		}
		work = checkedNext(t, e)
	}
	state, _ = e.load()
	run, _ := state.LatestStageRun()
	rounds := state.Rounds(run.ID)
	if len(state.StageRuns()) != 1 || len(rounds) != 3 || rounds[1].Decision.Assignee != "senior" || rounds[1].Decision.FixRule != 1 || rounds[2].Assignee != "senior" {
		t.Fatalf("reassignment did not retain the stage run: %+v", rounds)
	}
	if rounds[0].Decision.Assignee != plan.Worker || rounds[0].Decision.FixRule != 0 {
		t.Fatal("rule matched before two required failures")
	}
	base, err := e.stageDiffBase(rounds[2])
	if err != nil || base != first.StartTarget.Tree {
		t.Fatalf("comparison base changed: %s, %v", base, err)
	}
	paths, err := e.workspace.ChangedFiles(t.Context(), base, rounds[2].StartTarget.Tree)
	if err != nil || !slices.Equal(paths, []string{"first.txt", "second.txt"}) {
		t.Fatalf("lost cumulative diff: %v, %v", paths, err)
	}
}

func TestHumanRejectedFixRuleKeepsCumulativeDiff(t *testing.T) {
	rejected := true
	e, work := roleWork(t, func(stage *plan.Stage) {
		stage.Gate.Required = true
		stage.Transitions.FixRules = []plan.FixRule{{When: &plan.FixCondition{HumanRejected: &rejected}, Assignee: "senior"}}
	})
	write(t, e.Root(), "kept.txt", "Keep the earlier work.\n")
	submitWork(t, e, work)
	question := checkedNext(t, e)
	answerResult(t, e, question, ledger.AnswerRejected)
	next := checkedNext(t, e)
	state, _ := e.load()
	run, _ := state.LatestStageRun()
	rounds := state.Rounds(run.ID)
	if next.Instruction.Kind != form.InstructionWork || len(state.StageRuns()) != 1 || len(rounds) != 2 || rounds[0].Decision.Assignee != "senior" || rounds[0].Decision.FixRule != 1 || rounds[1].Assignee != "senior" {
		t.Fatalf("rejection did not reassign the fix: %+v", rounds)
	}
	paths, err := e.workspace.ChangedFiles(t.Context(), rounds[0].StartTarget.Tree, rounds[1].StartTarget.Tree)
	if err != nil || !slices.Contains(paths, "kept.txt") {
		t.Fatalf("rejection lost cumulative work: %v, %v", paths, err)
	}
}

func TestAgentWorkRoleRunsAsOneWorktreeUnit(t *testing.T) {
	e, result := roleWork(t, func(stage *plan.Stage) {
		stage.Assignee = "agent"
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	if len(result.Instructions) != 1 {
		t.Fatalf("expected one worktree instruction: %+v", result)
	}
	unit := result.Instructions[0]
	if unit.Kind != form.InstructionUnitWork || unit.Runner != "devkit-worker" || unit.TargetPath != e.unitPath(unit.ID) {
		t.Fatalf("wrong worktree runner: %+v", unit)
	}
	if !strings.Contains(briefText(t, &unit), "Role ID: agent") {
		t.Fatal("worktree brief omitted the role")
	}
	write(t, unit.TargetPath, "unit.txt", "Merged one-unit work.\n")
	if _, err := os.Stat(filepath.Join(e.Root(), "unit.txt")); !os.IsNotExist(err) {
		t.Fatal("unit edited the session checkout before merging")
	}
	collect(t, e, &unit, form.Progress{InstructionID: unit.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated the assigned unit"}})
	done := checkedNext(t, e)
	if done.Status != ledger.WorkCompleted {
		t.Fatalf("one-unit stage did not complete: %+v", done)
	}
	body, err := os.ReadFile(filepath.Join(e.Root(), "unit.txt"))
	if err != nil || string(body) != "Merged one-unit work.\n" {
		t.Fatalf("unit was not merged: %q, %v", body, err)
	}
	state, _ := e.load()
	rounds := state.Rounds(state.StageRuns()[0].ID)
	merge, ok := state.RoundMerge(rounds[0].ID)
	if len(rounds) != 1 || !ok || len(merge.Inputs) != 1 || merge.Status != "merged" {
		t.Fatalf("work role bypassed unit merging: %+v", merge)
	}
}

func TestPremiseBrokenAsksAndRecommendsHumanBack(t *testing.T) {
	e, work := roleWork(t, func(stage *plan.Stage) { stage.Checks = []plan.Check{judgeCheck("wording", true)} })
	submitWork(t, e, work)
	judgment, err := e.Next(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(judgment.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	result := pass(*data.Request)
	result.Judgment = form.Fail
	result.Findings = []form.Finding{{ID: "broken", Blocking: true, Kind: form.AutoFixable, Scenario: "An earlier premise no longer holds.", PremiseBroken: true}}
	collect(t, e, judgment.Instruction, result)
	question := checkedNext(t, e)
	q := question.Instruction.Question
	if q == nil || q.Recommendation != "back" || !slices.Contains(q.Options, "back") {
		t.Fatalf("broken premise did not recommend back: %+v", q)
	}
	state, _ := e.load()
	if state.StageRuns()[0].State == ledger.StageInvalidated {
		t.Fatal("the judge returned without a human decision")
	}
	answerResult(t, e, question, ledger.AnswerBack)
	state, _ = e.load()
	if state.StageRuns()[0].State != ledger.StageInvalidated {
		t.Fatal("human back did not use the existing return path")
	}
}

func TestLimitOffersRoleContinuationOnlyAfterHumanChoice(t *testing.T) {
	e, work := roleWork(t, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:fail", Runner: "command:fail", Required: true}}
		stage.Limits = plan.Limits{MaxRounds: 1, MaxNoProgress: 1}
		stage.Transitions.FixRules = []plan.FixRule{{Assignee: "senior"}}
	})
	submitWork(t, e, work)
	question := checkedNext(t, e)
	q := question.Instruction.Question
	if q == nil || q.StopCategory != "round_limit" || !slices.Contains(q.Options, "continue:senior") {
		t.Fatalf("limit did not offer a rule role: %+v", q)
	}
	state, _ := e.load()
	rounds := state.Rounds(state.StageRuns()[0].ID)
	if rounds[0].Decision.Assignee != "" || rounds[0].Decision.Action != plan.Ask {
		t.Fatal("rule overrode the round limit")
	}
	observeAnswer(t, e, "Continue with the senior role.")
	if _, err := e.Answer(t.Context(), AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "continue:senior"}); err != nil {
		t.Fatal(err)
	}
	work = checkedNext(t, e)
	state, _ = e.load()
	rounds = state.Rounds(state.StageRuns()[0].ID)
	if len(rounds) != 2 || rounds[1].Assignee != "senior" {
		t.Fatal("human role choice was not used in the same stage run")
	}
	submitWork(t, e, work)
	next := checkedNext(t, e)
	if next.Instruction.Question == nil || next.Instruction.Question.StopCategory != "round_limit" {
		t.Fatal("continued rounds bypassed the reset limit")
	}
}

func TestRolePlanValidation(t *testing.T) {
	settings := config.Config{}
	for _, stage := range []plan.Stage{
		{ID: "s", Assignee: "missing"},
		{ID: "s", Assignee: "judge"},
		{ID: "s", Assignee: "worker", Split: true},
		{ID: "s", Assignee: "worker", Checks: []plan.Check{{Runner: "planner"}}},
		{ID: "s", Assignee: "worker", Split: true, Units: []plan.Unit{{Role: "worker"}}},
		{ID: "s", Assignee: "worker", Transitions: plan.Transitions{FixRules: []plan.FixRule{{Assignee: "human"}}}},
	} {
		if err := validateRoles([]plan.Stage{stage}, settings); err == nil {
			t.Fatalf("accepted invalid roles: %+v", stage)
		}
	}
}

func TestUndoRoundsPreserveAssigneeWithoutRules(t *testing.T) {
	stage := plan.Stage{Assignee: plan.Worker, Transitions: plan.Transitions{FixRules: []plan.FixRule{{Assignee: "senior"}}}}
	decision, err := (&Engine{}).correctionDecision(nil, stage, &ledger.Round{Assignee: "agent"}, router.Decision{Action: plan.Fix, PreserveAssignee: true})
	if err != nil || decision.Assignee != "agent" || decision.FixRule != 0 {
		t.Fatalf("undo changed the assignee: %+v, %v", decision, err)
	}
}

func TestDefaultRolesPreserveInstructionsAndDeviations(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := e.settings(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for role, agent := range map[string]string{"worker": "", "agent": "devkit-worker", "judge": "devkit-judge", "planner": "devkit-planner", "human": ""} {
		got, err := e.runnerAgent(ctx, "HEAD", role)
		if err != nil || got != agent {
			t.Fatalf("default role %s changed instruction runner: %q, %v", role, got, err)
		}
	}
	for _, name := range []string{"default", "fix", "docs"} {
		template, source, err := e.template(ctx, "HEAD", name)
		if err != nil {
			t.Fatal(err)
		}
		p := plan.NewPlan(template, plan.ContentHash(source), "Review the default roles", []plan.CompletionCondition{{ID: "C1", Description: "Preserve default behavior"}})
		if err := validateRoles(p.Stages, settings); err != nil {
			t.Fatal(err)
		}
		if deviations := plan.Deviations(template, p); len(deviations) != 0 {
			t.Fatalf("default roles introduced deviations: %+v", deviations)
		}
		for _, stage := range p.Stages {
			if stage.Assignee != plan.Worker || stage.Transitions.FixRules != nil {
				t.Fatal("default stage work changed")
			}
			for _, item := range stage.Checks {
				if !item.Runner.IsCommand() && item.Runner != "judge" {
					t.Fatal("default judgment role changed")
				}
			}
		}
	}
	started, err := e.Start(ctx, StartRequest{Template: "default", Task: "Preserve default instructions"})
	if err != nil {
		t.Fatal(err)
	}
	if started.Instruction.Runner != "devkit-planner" || started.Instruction.Text != "Launch the planner with: devkit-instruction "+started.Instruction.ID {
		t.Fatal("default planner launch changed")
	}
	template, source, err := e.template(ctx, "HEAD", "default")
	if err != nil {
		t.Fatal(err)
	}
	p := plan.NewPlan(template, plan.ContentHash(source), "Preserve default instructions", []plan.CompletionCondition{{ID: "C1", Description: "Agree on requirements"}})
	for i := range p.Stages {
		for j := range p.Stages[i].Checks {
			p.Stages[i].Checks[j].CompletionConditionIDs = []string{"C1"}
			p.Stages[i].Checks[j].Target = "Task outputs"
		}
	}
	collect(t, e, started.Instruction, form.PlannerResult{InstructionID: started.Instruction.ID, Plan: marshal(t, p)})
	work := checkedNext(t, e)
	if work.Instruction.Kind != form.InstructionWork || work.Instruction.Text != "Perform the work described in the brief, then run devkit submit." {
		t.Fatalf("default work instruction changed: %+v", work)
	}
	state, _ := e.load()
	stage := state.StageRuns()[0]
	if stage.StageID != "purpose" || !reflect.DeepEqual(state.Rounds(stage.ID)[0].Assignee, plan.Worker) {
		t.Fatal("default work assignment changed")
	}
}

func roleNext(t *testing.T, e *Engine) Result {
	t.Helper()
	for range 20 {
		result, err := e.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Instructions) > 0 {
			return result
		}
		if result.Instruction == nil {
			continue
		}
		switch result.Instruction.Kind {
		case form.InstructionDetectionTest:
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
	t.Fatal("role instructions did not terminate")
	return Result{}
}

func TestRoleChangesAppearInPlanApproval(t *testing.T) {
	e, ctx := fixture(t)
	if _, err := e.Start(ctx, StartRequest{Template: "test", Task: "Review role changes"}); err != nil {
		t.Fatal(err)
	}
	template, source, err := e.template(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	p := plan.NewPlan(template, plan.ContentHash(source), "Review role changes", []plan.CompletionCondition{{ID: "C1", Description: "Correct assignments"}})
	p.Stages[0].Assignee = "agent"
	p.Stages[0].Transitions.FixRules = []plan.FixRule{{Assignee: "worker"}}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	text, err := e.planApprovalText(state, template, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"changed_assignee", "changed_fix_rules", "Template: \"worker\"", "Plan: \"agent\"", "\"assignee\": \"worker\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("approval omitted %q: %s", want, text)
		}
	}
}

func TestStartRejectsWritableOutsideSource(t *testing.T) {
	e, ctx := fixture(t)
	outside := t.TempDir()
	configure(t, e, "[sources.extra]\npath = "+strconv.Quote(outside)+"\nwritable=true\n")
	_, err := e.Start(ctx, StartRequest{Template: "test", Task: "Read outside knowledge"})
	if err == nil || !strings.Contains(err.Error(), "outside the repository are read-only") {
		t.Fatalf("outside writable source was accepted: %v", err)
	}
	if e.workID != "" {
		t.Fatal("configuration error created work")
	}
}

func TestRoleConfigurationAndAgentChangesAreNorms(t *testing.T) {
	e, ctx := fixture(t)
	before, err := e.norms(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[roles.senior]\nkind='work'\nagent='devkit-senior'\n")
	for _, host := range []string{"claude", "codex"} {
		extension := ".md"
		if host == "codex" {
			extension = ".toml"
		}
		body, err := os.ReadFile(filepath.Join(e.Root(), "."+host, "agents", "devkit-worker"+extension))
		if err != nil {
			t.Fatal(err)
		}
		write(t, e.Root(), "."+host+"/agents/devkit-senior"+extension, strings.ReplaceAll(string(body), "devkit-worker", "devkit-senior"))
	}
	git(t, e.Root(), "add", "devkit.toml")
	git(t, e.Root(), "commit", "-qm", "Configure senior role")
	if _, err := e.Start(ctx, StartRequest{Template: "test", Task: "Use a repository role"}); err == nil || !strings.Contains(err.Error(), "commit the devkit init changes first") {
		t.Fatalf("uncommitted custom definitions were accepted: %v", err)
	}
	git(t, e.Root(), "add", ".claude/agents/devkit-senior.md", ".codex/agents/devkit-senior.toml")
	git(t, e.Root(), "commit", "-qm", "Commit senior agent definitions")
	after, err := e.norms(ctx, "HEAD")
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatalf("role changes did not affect norm identity: %v", err)
	}
	if !slices.Contains(differenceCategories(e.Root(), []string{"devkit.toml", ".claude/agents/devkit-senior.md"}, config.Config{}), config.MetaLoopEdit) {
		t.Fatal("role edits were not classified as loop edits")
	}
	if _, err := e.Start(ctx, StartRequest{Template: "test", Task: "Use a repository role"}); err != nil {
		t.Fatal(err)
	}
}

func TestAgentWorkRoleFixUsesAnotherWorktreeUnit(t *testing.T) {
	e, result := roleWork(t, func(stage *plan.Stage) {
		stage.Assignee = "agent"
		stage.Checks = []plan.Check{judgeCheck("wording", true)}
	})
	unit := result.Instructions[0]
	write(t, unit.TargetPath, "kept.txt", "Keep the merged first-round work.\n")
	collect(t, e, &unit, form.Progress{InstructionID: unit.ID, TerminationKind: form.TerminationCompleted})
	judgment, err := e.Next(t.Context())
	if err != nil || judgment.Instruction == nil || judgment.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("unit did not reach judgment: %+v, %v", judgment, err)
	}
	data, err := e.readInstruction(judgment.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	failure := pass(*data.Request)
	failure.Judgment = form.Fail
	failure.Findings = []form.Finding{{ID: "fix", Blocking: true, Kind: form.AutoFixable, Scenario: "Revise the wording without discarding the first change."}}
	collect(t, e, judgment.Instruction, failure)
	next := roleNext(t, e)
	if len(next.Instructions) != 1 || next.Instructions[0].Runner != "devkit-worker" || next.Instructions[0].TargetPath == unit.TargetPath {
		t.Fatalf("fix did not use a new worktree unit: %+v", next)
	}
	body, err := os.ReadFile(filepath.Join(next.Instructions[0].TargetPath, "kept.txt"))
	if err != nil || string(body) != "Keep the merged first-round work.\n" {
		t.Fatalf("fix lost the merged work: %q, %v", body, err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	rounds := state.Rounds(state.StageRuns()[0].ID)
	if len(rounds) != 2 || rounds[1].MergeFix || rounds[1].Assignee != "agent" {
		t.Fatalf("fix bypassed its assigned work role: %+v", rounds)
	}
}

func TestOutsideSourceHasNoEditableCopyOrWriteback(t *testing.T) {
	e, products, guides, work := outsideSourceWork(t)
	if !strings.Contains(briefText(t, work.Instruction), "Read-only source; do not edit it or write it back") {
		t.Fatal("outside input was not marked read-only in the work brief")
	}
	if _, err := os.Stat(filepath.Join(e.workRoot(), "sources", "edit")); !os.IsNotExist(err) {
		t.Fatalf("an outside editable copy exists: %v", err)
	}
	submitWork(t, e, work)
	result := checkedNext(t, e)
	if result.Status != ledger.WorkCompleted {
		t.Fatalf("read-only outside inputs did not complete: %+v", result)
	}
	for name, want := range map[string]string{filepath.Join(products, "requirements.md"): "Initial outside requirements.\n", filepath.Join(guides, "style.md"): "Initial outside guide.\n"} {
		body, err := os.ReadFile(name)
		if err != nil || string(body) != want {
			t.Fatalf("outside input changed: %s: %q, %v", name, body, err)
		}
	}
}

func TestWorkerQuestionRejectionUsesFixRules(t *testing.T) {
	rejected := true
	e, work := roleWork(t, func(stage *plan.Stage) {
		stage.Transitions.FixRules = []plan.FixRule{{When: &plan.FixCondition{HumanRejected: &rejected}, Assignee: "senior"}}
	})
	if _, err := e.Submit(t.Context(), marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Confirm how to continue."), Obstacles: []string{"Confirm the next edit."}})); err != nil {
		t.Fatal(err)
	}
	asked := checkedNext(t, e)
	answerResult(t, e, asked, ledger.AnswerRejected)
	checkedNext(t, e)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	rounds := state.Rounds(state.StageRuns()[0].ID)
	if len(rounds) != 2 || rounds[0].Decision.FixRule != 1 || rounds[1].Assignee != "senior" {
		t.Fatalf("worker confirmation bypassed reassignment: %+v", rounds)
	}
}

func TestUnassessedContinuationAppliesFixRule(t *testing.T) {
	e, work := roleWork(t, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", true)}
		stage.Transitions.FixRules = []plan.FixRule{{When: &plan.FixCondition{Check: "wording"}, Assignee: "senior"}}
	})
	submitWork(t, e, work)
	judge, err := e.Next(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(judge.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	result := pass(*data.Request)
	result.Judgment = form.NotAssessed
	result.CompletionConditionIDs = []string{}
	collect(t, e, judge.Instruction, result)
	asked := checkedNext(t, e)
	answerResult(t, e, asked, ledger.AnswerText)
	checkedNext(t, e)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	rounds := state.Rounds(state.StageRuns()[0].ID)
	if len(rounds) != 2 || rounds[0].Decision.FixRule != 1 || rounds[1].Assignee != "senior" {
		t.Fatalf("unassessed continuation bypassed fix rules: %+v", rounds)
	}
}
