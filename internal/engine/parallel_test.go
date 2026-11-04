package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func splitWork(t *testing.T, maxWorkers int, extraUnits ...plan.Unit) (*Engine, context.Context, Result) {
	t.Helper()
	// Units default to the worker subagent; the split stage is assigned to a session work role.
	e, ctx := defaultFixture(t)
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	template.Stages[0].Split = true
	template.Stages[0].Assignee = "session"
	template.Stages[0].Checks = []plan.Check{judgeCheck("plan", true)}
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	extra := "\n[detection]\nrequired = false\n[roles.session]\nkind = 'work'\nagent = ''\n"
	if maxWorkers == 1 {
		extra += "\n[parallel]\nmax_concurrent = 1\n"
	}
	write(t, e.Root(), "devkit.toml", string(settings)+extra)
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Enable split work")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update two documents"})
	if err != nil {
		t.Fatal(err)
	}
	source, err = os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := newPlan(template, plan.ContentHash(source), "Update two documents", []plan.CompletionCondition{{ID: "C1", Description: "Both documents are updated"}})
	p.Stages[0].Units = []plan.Unit{{ID: "first/unit", Description: "Update the first document", Scope: "first.md"}, {ID: "second", Description: "Update the second document", Scope: "second.md"}}
	p.Stages[0].Units = append(p.Stages[0].Units, extraUnits...)
	p.Stages[0].Checks[0].Target = "Documents"
	p.Stages[0].Checks[0].CompletionConditionIDs = []string{"C1"}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionPlanCheck {
		t.Fatalf("expected a plan check: %+v", review)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, review.Instruction, pass(*data.Request))
	return e, ctx, nextResult(t, e, ctx)
}

func nextResult(t *testing.T, e *Engine, ctx context.Context) Result {
	t.Helper()
	r, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return approvedNext(t, e, r)
}

func finishUnit(t *testing.T, e *Engine, i form.Instruction, file, body string) {
	t.Helper()
	write(t, i.TargetPath, file, body)
	collect(t, e, &i, form.Progress{InstructionID: i.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated " + file}, Confirmed: []string{"Read the unit scope"}})
}

func TestSplitStageEntrypointMergesAndJudgesOneRound(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	if len(work.Instructions) != 2 || work.Instruction != nil {
		t.Fatalf("expected two worker instructions: %+v", work)
	}
	before := git(t, e.Root(), "rev-parse", "HEAD")
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	for _, i := range work.Instructions {
		if i.Kind != form.InstructionUnitWork || i.Runner != "devkit-worker" || i.TargetPath != e.unitPath(i.ID) {
			t.Fatalf("invalid unit instruction: %+v", i)
		}
		if git(t, i.TargetPath, "rev-parse", "HEAD") != round.StartCommit || git(t, i.TargetPath, "rev-parse", "--abbrev-ref", "HEAD") != "HEAD" {
			t.Fatal("unit worktree is not detached at the round snapshot")
		}
		body, err := os.ReadFile(i.Brief)
		if err != nil || !strings.Contains(string(body), i.TargetPath) || !strings.Contains(string(body), "Scope:") {
			t.Fatalf("worker brief omits its absolute worktree or scope: %v", err)
		}
	}
	repeated := nextResult(t, e, ctx)
	if len(repeated.Instructions) != 2 || repeated.Instructions[0].ID != work.Instructions[0].ID {
		t.Fatal("next changed outstanding unit instructions")
	}
	h := hook.Handler{Root: e.Root(), WorkID: e.workID, Validator: e.validator}
	stopped, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.Stop, SessionID: e.sessionID})
	if err != nil || stopped.Decision == event.Block {
		t.Fatalf("Stop did not allow waiting for workers: %+v: %v", stopped, err)
	}
	finishUnit(t, e, work.Instructions[0], "first.md", "First unit output\n")
	finishUnit(t, e, work.Instructions[1], "second.md", "Second unit output\n")
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("expected one stage judgment: %+v", review)
	}
	if git(t, e.Root(), "rev-parse", "HEAD") != before {
		t.Fatal("merging units created a branch commit before judgment")
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data.Request.Progress == nil || len(data.Request.Progress.WorkPerformed) != 2 {
		t.Fatal("stage progress does not combine both units")
	}
	for _, file := range []string{"first.md", "second.md"} {
		body, err := os.ReadFile(filepath.Join(e.Root(), file))
		if err != nil || !strings.Contains(string(body), "unit output") {
			t.Fatalf("unit output was not merged: %s: %v", file, err)
		}
		if _, err := os.Stat(filepath.Join(data.Instruction.TargetPath, file)); err != nil {
			t.Fatalf("judgment target omits %s: %v", file, err)
		}
	}
	brief, err := os.ReadFile(review.Instruction.Brief)
	if err != nil || !strings.Contains(string(brief), "first.md") || !strings.Contains(string(brief), "second.md") || !strings.Contains(string(brief), round.StartTarget.Tree) {
		t.Fatalf("stage judgment omits cumulative diff: %v", err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	merge, ok := state.RoundMerge(round.ID)
	if !ok || merge.Status != "merged" || merge.Head != before || len(merge.Inputs) != 2 || len(state.UnitResults(round.ID)) != 2 {
		t.Fatalf("unit results and fixed SHAs were not recorded: %+v", merge)
	}
	for _, input := range merge.Inputs {
		if git(t, e.Root(), "cat-file", "-t", input.Commit) != "commit" {
			t.Fatal("merge input was not fixed as a commit")
		}
	}
	// A late edit in a worker checkout must not alter the already fixed judgment target.
	write(t, work.Instructions[0].TargetPath, "first.md", "Late worker edit\n")
	collect(t, e, review.Instruction, pass(*data.Request))
	done := nextResult(t, e, ctx)
	if done.Status != ledger.WorkCompleted || done.Instruction.Kind != form.InstructionCompleted {
		t.Fatalf("stage did not complete: %+v", done)
	}
	if git(t, e.Root(), "show", "HEAD:first.md") != "First unit output" || !strings.Contains(done.Warning, work.Instructions[0].TargetPath) {
		t.Fatal("completion did not use the fixed merged tree or report retained worktrees")
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	if len(state.Rounds(run.ID)) != 1 || run.Commit == before || run.State != ledger.StageCompleted {
		t.Fatalf("split work did not use one stage round and completion commit: %+v", run)
	}
}

func TestSplitMergeConflictReturnsSessionFixRound(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	finishUnit(t, e, work.Instructions[0], "README.md", "First conflicting output\n")
	finishUnit(t, e, work.Instructions[1], "README.md", "Second conflicting output\n")
	fix := nextResult(t, e, ctx)
	if fix.Instruction == nil || fix.Instruction.Kind != form.InstructionWork || len(fix.Instructions) != 0 {
		t.Fatalf("conflict did not return to the session worker: %+v", fix)
	}
	body, err := os.ReadFile(fix.Instruction.Brief)
	if err != nil || !strings.Contains(string(body), "README.md") || !strings.Contains(string(body), "first/unit") || !strings.Contains(string(body), "second") {
		t.Fatalf("fix brief omits conflicting paths and units: %v", err)
	}
	write(t, e.Root(), "README.md", "Resolved unit outputs\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: fix.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("resolved merge was not judged: %+v", review)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, review.Instruction, pass(*data.Request))
	if done := nextResult(t, e, ctx); done.Status != ledger.WorkCompleted {
		t.Fatalf("resolved merge did not complete: %+v", done)
	}
}

func TestSplitMaxWorkersOne(t *testing.T) {
	e, ctx, work := splitWork(t, 1)
	if len(work.Instructions) != 1 {
		t.Fatalf("max_workers=1 issued %d instructions", len(work.Instructions))
	}
	first := work.Instructions[0]
	if repeated := nextResult(t, e, ctx); len(repeated.Instructions) != 1 || repeated.Instructions[0].ID != first.ID {
		t.Fatal("issued a second worker while the first was outstanding")
	}
	finishUnit(t, e, first, "first.md", "First sequential output\n")
	second := nextResult(t, e, ctx)
	if len(second.Instructions) != 1 || second.Instructions[0].ID == first.ID || second.Instructions[0].TargetPath == first.TargetPath {
		t.Fatalf("did not issue the next isolated unit: %+v", second)
	}
	finishUnit(t, e, second.Instructions[0], "second.md", "Second sequential output\n")
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("sequential units did not merge: %+v", review)
	}
}

func TestSplitPauseWhileWorkersRun(t *testing.T) {
	e, ctx, work := splitWork(t, 1)
	if _, err := e.Pause(ctx, "Wait for human review"); err != nil {
		t.Fatal(err)
	}
	finishUnit(t, e, work.Instructions[0], "first.md", "Output during pause\n")
	paused := nextResult(t, e, ctx)
	if !paused.Paused || paused.Instruction != nil || len(paused.Instructions) != 0 {
		t.Fatalf("pause issued new work: %+v", paused)
	}
	results, err := filepath.Glob(filepath.Join(e.workRoot(), "results", work.Instructions[0].ID+"-*.json"))
	if err != nil || len(results) != 1 {
		t.Fatal("running worker result was not preserved during pause")
	}
	if _, err := e.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	next := nextResult(t, e, ctx)
	if len(next.Instructions) != 1 || next.Instructions[0].ID == work.Instructions[0].ID {
		t.Fatalf("resume did not accept the preserved result and issue the next unit: %+v", next)
	}
}

func TestSplitAbortKeepsChangedWorktrees(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	changed, clean := work.Instructions[0], work.Instructions[1]
	write(t, changed.TargetPath, "unfinished.md", "Keep this work\n")
	aborted, err := e.Abort(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if aborted.Status != ledger.WorkAborted || !strings.Contains(aborted.Warning, changed.TargetPath) {
		t.Fatalf("abort did not report retained work: %+v", aborted)
	}
	if _, err := os.Stat(filepath.Join(changed.TargetPath, "unfinished.md")); err != nil {
		t.Fatal("abort discarded unit changes")
	}
	if _, err := os.Stat(clean.TargetPath); !os.IsNotExist(err) {
		t.Fatalf("abort retained an unchanged worktree: %v", err)
	}
}

func TestSplitMalformedResultWithoutIDAsksHuman(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	h := hook.Handler{Root: e.Root(), WorkID: e.workID, Validator: e.validator, InstructionRunner: e.InstructionRunner}
	if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.SubagentStop, SessionID: e.sessionID, AgentID: "ambiguous-worker", AgentType: "devkit-worker", LastAssistantMessage: "Invalid worker result"}); err != nil {
		t.Fatal(err)
	}
	question := nextResult(t, e, ctx)
	if question.Instruction == nil || question.Instruction.Question == nil || question.Instruction.Question.StopCategory != "ambiguous_result" {
		t.Fatalf("unassociated result was not escalated: %+v", question)
	}
	q := question.Instruction.Question
	observeAnswer(t, e, "Retry instruction "+work.Instructions[0].ID)
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Retry the failed worker"}); err == nil {
		t.Fatal("accepted an ambiguous answer without an instruction ID")
	}
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: work.Instructions[0].ID}); err != nil {
		t.Fatal(err)
	}
	finishUnit(t, e, work.Instructions[1], "second.md", "Second output\n")
	retry := nextResult(t, e, ctx)
	if len(retry.Instructions) != 1 || retry.Instructions[0].ID == work.Instructions[0].ID || retry.Instructions[0].Reissue != 1 || retry.Instructions[0].TargetPath != work.Instructions[0].TargetPath {
		t.Fatalf("the human-selected worker was not reissued in its existing worktree: %+v", retry)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	answered, rejected := false, false
	for _, record := range records {
		switch value := record.Payload.(type) {
		case *ledger.Answer:
			answered = answered || value.QuestionID == q.ID && value.Text == work.Instructions[0].ID
		case *ledger.ResultRejection:
			rejected = rejected || value.InstructionID == work.Instructions[0].ID
		}
	}
	if !answered || !rejected {
		t.Fatal("the ambiguous answer or rejection was not recorded")
	}
	finishUnit(t, e, retry.Instructions[0], "first.md", "Retried output\n")
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("reissued unit did not merge: %+v", review)
	}
}

func TestSplitRecoversAppliedMergeBeforeCheckpoint(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	finishUnit(t, e, work.Instructions[0], "first.md", "First fixed output\n")
	finishUnit(t, e, work.Instructions[1], "second.md", "Second fixed output\n")
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if changed, err := e.acceptPending(ctx, state); err != nil || !changed {
			t.Fatalf("unit result was not accepted: %v", err)
		}
		state, err = e.load()
		if err != nil {
			t.Fatal(err)
		}
	}
	round, _ := state.OpenRound()
	bound := state.CurrentPosition().Plan.Plan
	stage, _ := bound.Stage("implement")
	merge, err := e.freezeUnits(ctx, state, stage, round)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.mergeUnit(ctx, round, merge); err != nil {
		t.Fatal(err)
	}
	if err := e.workspace.Merge(ctx, round.StartCommit, merge.Current, merge.Inputs[0].Commit); err != nil {
		t.Fatal(err)
	}
	// The merge changed the checkout, but its successful checkpoint has not been saved.
	write(t, work.Instructions[1].TargetPath, "second.md", "Late unfixed output\n")
	resumed, err := newTestEngine(ctx, e.Root(), e.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	review := nextResult(t, resumed, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment || git(t, e.Root(), "rev-parse", "HEAD") != merge.Head {
		t.Fatalf("interrupted merge did not recover without moving HEAD: %+v", review)
	}
	body, err := os.ReadFile(filepath.Join(e.Root(), "second.md"))
	if err != nil || string(body) != "Second fixed output\n" {
		t.Fatalf("recovery reread a changed worker tree: %s: %v", body, err)
	}
}

func TestSplitConfigConflictUsesFrozenNormsForRepair(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	original := git(t, e.Root(), "show", "HEAD:devkit.toml") + "\n"
	finishUnit(t, e, work.Instructions[0], "devkit.toml", "default_plan = 'first'\n"+original)
	finishUnit(t, e, work.Instructions[1], "devkit.toml", "default_plan = 'second'\n"+original)
	fix := nextResult(t, e, ctx)
	if fix.Instruction == nil || fix.Instruction.Kind != form.InstructionWork {
		t.Fatalf("conflicting configuration did not return to the worker: %+v", fix)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	if roundReference(round) == round.StartTarget.Tree {
		t.Fatal("repair used conflicted norms")
	}
	write(t, e.Root(), "devkit.toml", original)
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: fix.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("repaired configuration was not judged: %+v", review)
	}
}

func TestSplitConflictPreservesUnitConfirmation(t *testing.T) {
	e, ctx, work := splitWork(t, 3)
	first := work.Instructions[0]
	write(t, first.TargetPath, "README.md", "First conflicting output\n")
	collect(t, e, &first, form.Progress{
		InstructionID: first.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Confirm how to continue."),
		WorkPerformed:    []string{"Updated the first unit"},
		Obstacles:        []string{"Choose the supported document format"},
		Unconfirmed:      []string{"Human confirmation is still required"},
		ResumeConditions: []string{"The human chooses a format"},
	})
	finishUnit(t, e, work.Instructions[1], "README.md", "Second conflicting output\n")
	fix := nextResult(t, e, ctx)
	if fix.Instruction == nil || fix.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected a conflict repair: %+v", fix)
	}
	beforeRepair, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	aggregate, ok := beforeRepair.LatestProgress()
	if !ok || aggregate.TerminationKind != form.TerminationQuestion || len(aggregate.WorkPerformed) != 2 || len(aggregate.Obstacles) != 1 {
		t.Fatalf("conflicting units were not aggregated before repair: %+v", aggregate)
	}
	write(t, e.Root(), "README.md", "Resolved unit outputs\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: fix.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Resolved the conflict"}})); err != nil {
		t.Fatal(err)
	}
	question := nextResult(t, e, ctx)
	if question.Instruction == nil || question.Instruction.Question == nil || question.Instruction.Question.StopCategory != "worker_confirmation" {
		t.Fatalf("unit confirmation was bypassed after conflict resolution: %+v", question)
	}
	q := question.Instruction.Question
	if !strings.Contains(q.Text, "Choose the supported document format") || !strings.Contains(q.Text, "Human confirmation is still required") {
		t.Fatalf("unit confirmation details were lost: %s", q.Text)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	progress, ok := state.LatestProgress()
	if run.State != ledger.StageAnswerWait || !ok || progress.TerminationKind != form.TerminationQuestion || len(progress.WorkPerformed) != 3 || len(progress.ResumeConditions) != 1 {
		t.Fatalf("unit termination blocks were not retained as stage state: %+v, %+v", run, progress)
	}
	observeAnswer(t, e, "Use Markdown")
	if _, err := e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Use Markdown"}); err != nil {
		t.Fatal(err)
	}
	continued := nextResult(t, e, ctx)
	if continued.Instruction == nil || continued.Instruction.Kind != form.InstructionWork {
		t.Fatalf("human guidance did not return to the worker: %+v", continued)
	}
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: continued.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	review := nextResult(t, e, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("answered unit confirmation was not resolved: %+v", review)
	}
}

func TestSplitMiddleConflictResumesRemainingFixedInput(t *testing.T) {
	e, ctx, work := splitWork(t, 3, plan.Unit{ID: "third", Description: "Update the third document", Scope: "third.md"})
	if len(work.Instructions) != 3 {
		t.Fatalf("expected three unit instructions: %+v", work)
	}
	finishUnit(t, e, work.Instructions[0], "README.md", "First conflicting output\n")
	finishUnit(t, e, work.Instructions[1], "README.md", "Second conflicting output\n")
	finishUnit(t, e, work.Instructions[2], "third.md", "Third fixed output\n")
	fix := nextResult(t, e, ctx)
	if fix.Instruction == nil || fix.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected a repair for the middle input: %+v", fix)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	original := state.Rounds(run.ID)[0]
	conflict, _ := state.RoundMerge(original.ID)
	if conflict.Status != "conflict" || conflict.Applied != 1 {
		t.Fatalf("expected a conflict on the middle input: %+v", conflict)
	}
	if _, err := os.Stat(filepath.Join(e.Root(), "third.md")); !os.IsNotExist(err) {
		t.Fatalf("third input was already applied: %v", err)
	}
	write(t, work.Instructions[2].TargetPath, "third.md", "Late unfixed output\n")
	write(t, e.Root(), "README.md", "Resolved first and second outputs\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: fix.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	resumed, err := newTestEngine(ctx, e.Root(), e.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	review := nextResult(t, resumed, ctx)
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("remaining fixed input was not merged before judgment: %+v", review)
	}
	body, err := os.ReadFile(filepath.Join(e.Root(), "third.md"))
	if err != nil || string(body) != "Third fixed output\n" {
		t.Fatalf("remaining input was lost or reread from its worker: %s: %v", body, err)
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	merge, ok := state.RoundMerge(round.ID)
	if !ok || merge.Status != "merged" || merge.Applied != 3 || merge.Inputs[2] != conflict.Inputs[2] {
		t.Fatalf("completion lacks a merge record for every fixed input: %+v", merge)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, resumed, review.Instruction, pass(*data.Request))
	if done := nextResult(t, resumed, ctx); done.Status != ledger.WorkCompleted {
		t.Fatalf("all merged inputs did not complete: %+v", done)
	}
	if git(t, e.Root(), "show", "HEAD:third.md") != "Third fixed output" {
		t.Fatal("completion commit omitted the remaining fixed input")
	}
}

func TestSplitChecksCheckoutAfterFinalMergeCheckpoint(t *testing.T) {
	for _, commitEdit := range []bool{false, true} {
		name := "working tree edit"
		if commitEdit {
			name = "HEAD edit"
		}
		t.Run(name, func(t *testing.T) {
			e, ctx, work := splitWork(t, 3)
			finishUnit(t, e, work.Instructions[0], "first.md", "First fixed output\n")
			finishUnit(t, e, work.Instructions[1], "second.md", "Second fixed output\n")
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if changed, err := e.acceptPending(ctx, state); err != nil || !changed {
					t.Fatalf("unit result was not accepted: %v", err)
				}
				state, err = e.load()
				if err != nil {
					t.Fatal(err)
				}
			}
			round, _ := state.OpenRound()
			p := state.CurrentPosition().Plan.Plan
			stage, _ := p.Stage("implement")
			merge, err := e.freezeUnits(ctx, state, stage, round)
			if err != nil {
				t.Fatal(err)
			}
			for merge.Status != "merged" {
				if err := e.mergeUnit(ctx, round, merge); err != nil {
					t.Fatal(err)
				}
				state, err = e.load()
				if err != nil {
					t.Fatal(err)
				}
				merge, _ = state.RoundMerge(round.ID)
			}
			if commitEdit {
				git(t, e.Root(), "add", ".")
				git(t, e.Root(), "commit", "-qm", "Unexpected parent commit")
				if tree := git(t, e.Root(), "rev-parse", "HEAD^{tree}"); tree != merge.ResultTree {
					t.Fatal("the HEAD-only change also changed the recorded result tree")
				}
			} else {
				write(t, e.Root(), "late.md", "Unexpected parent edit\n")
			}
			resumed, err := newTestEngine(ctx, e.Root(), e.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			fix := nextResult(t, resumed, ctx)
			if fix.Instruction == nil || fix.Instruction.Kind != form.InstructionWork {
				t.Fatalf("changed checkout was silently submitted: %+v", fix)
			}
			brief, err := os.ReadFile(fix.Instruction.Brief)
			if err != nil || !strings.Contains(string(brief), "recorded unit merge") || !strings.Contains(string(brief), merge.Head) {
				t.Fatalf("repair brief omits the checkout difference: %v", err)
			}
			if !commitEdit {
				if !strings.Contains(string(brief), "late.md") {
					t.Fatal("repair brief omits the differing file")
				}
				if err := os.Remove(filepath.Join(e.Root(), "late.md")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := resumed.Submit(ctx, marshal(t, form.Progress{InstructionID: fix.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
				t.Fatal(err)
			}
			if review := nextResult(t, resumed, ctx); review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
				t.Fatalf("explicit checkout repair was not judged: %+v", review)
			}
		})
	}
}

func TestSplitNewSessionReissuesUnitsAndCollectsMalformedResult(t *testing.T) {
	e, ctx, work := splitWork(t, 1)
	first := work.Instructions[0]
	write(t, first.TargetPath, "first.md", "Preserved unit work\n")
	if err := hook.PublishSession(e.runRoot(), hook.SessionObservation{Host: event.ClaudeCode, SessionID: "session-two", ObservedAt: time.Now().UTC()}, "SessionStart"); err != nil {
		t.Fatal(err)
	}
	second, err := newTestEngine(ctx, e.Root(), "session-two")
	if err != nil {
		t.Fatal(err)
	}
	resumed := nextResult(t, second, ctx)
	if len(resumed.Instructions) != 1 || resumed.Instructions[0].ID == first.ID || resumed.Instructions[0].Reissue != 1 || resumed.Instructions[0].TargetPath != first.TargetPath {
		t.Fatalf("session change did not reissue the unit in its existing worktree: %+v", resumed)
	}
	state, err := second.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	pending := state.UnacceptedInstructions()
	if round.SessionID != second.sessionID || len(pending) != 1 || pending[0].SessionID != second.sessionID {
		t.Fatal("reissued unit or round retained the old session binding")
	}
	brief, err := os.ReadFile(resumed.Instructions[0].Brief)
	if err != nil || !strings.Contains(string(brief), first.TargetPath) {
		t.Fatalf("reissued brief lost its worktree: %v", err)
	}
	h := hook.Handler{Root: second.Root(), WorkID: second.workID, Validator: second.validator, InstructionRunner: second.InstructionRunner}
	if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.SubagentStop, SessionID: second.sessionID, AgentID: "malformed-worker", AgentType: "devkit-worker", LastAssistantMessage: "Malformed worker result"}); err != nil {
		t.Fatal(err)
	}
	retry := nextResult(t, second, ctx)
	if len(retry.Instructions) != 1 || retry.Instructions[0].Reissue != 2 || retry.Instructions[0].ID == resumed.Instructions[0].ID || retry.Instructions[0].TargetPath != first.TargetPath {
		t.Fatalf("malformed result was dropped or failed to reissue: %+v", retry)
	}
	body, err := os.ReadFile(filepath.Join(retry.Instructions[0].TargetPath, "first.md"))
	if err != nil || string(body) != "Preserved unit work\n" {
		t.Fatalf("reissue discarded existing unit work: %s: %v", body, err)
	}
	finishUnit(t, second, retry.Instructions[0], "first.md", "Preserved unit work\n")
	next := nextResult(t, second, ctx)
	if len(next.Instructions) != 1 {
		t.Fatalf("expected the next unit: %+v", next)
	}
	finishUnit(t, second, next.Instructions[0], "second.md", "Second output\n")
	if review := nextResult(t, second, ctx); review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("reissued unit results did not merge: %+v", review)
	}
}
