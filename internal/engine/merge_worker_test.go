package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestDefaultSplitWorkerRepairsConflictedMergeInOneUnit(t *testing.T) {
	e, ctx := defaultFixture(t)
	work := flowWork(t, e, "", func(stage *plan.Stage) {
		stage.Split = true
		stage.Checks = []plan.Check{judgeCheck("wording", true)}
	}, func(p *plan.Plan) {
		p.Stages[0].Units = []plan.Unit{
			{ID: "first", Description: "First document update", Scope: "README.md"},
			{ID: "second", Description: "Second document update", Scope: "README.md"},
			{ID: "third", Description: "Independent update", Scope: "third.md"},
		}
	})
	if len(work.Instructions) != 3 {
		t.Fatalf("default split stage did not issue three units: %+v", work)
	}
	before := git(t, e.Root(), "rev-parse", "HEAD")
	first := work.Instructions[0]
	write(t, first.TargetPath, "README.md", "First conflicting output\n")
	collect(t, e, &first, form.Progress{InstructionID: first.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated README.md"}, Report: json.RawMessage(`{"result":"original-unit-report-marker"}`)})
	finishUnit(t, e, work.Instructions[1], "README.md", "Second conflicting output\n")
	finishUnit(t, e, work.Instructions[2], "third.md", "Third fixed output\n")
	fix := nextResult(t, e, ctx)
	if fix.Instruction != nil || len(fix.Instructions) != 1 {
		t.Fatalf("conflict repair did not issue one unit: %+v", fix)
	}
	repair := fix.Instructions[0]
	if repair.Kind != form.InstructionUnitWork || repair.Runner != "devkit-worker" || repair.TargetPath == e.Root() {
		t.Fatalf("repair did not use the default worker worktree: %+v", repair)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	rounds := state.Rounds(run.ID)
	if len(rounds) != 2 || !rounds[1].MergeFix || rounds[1].Assignee != plan.Worker {
		t.Fatalf("repair lost its stage assignee or round: %+v", rounds)
	}
	original, _ := state.RoundMerge(rounds[0].ID)
	if original.Status != "conflict" || original.Applied != 1 {
		t.Fatalf("expected the middle merge input to conflict: %+v", original)
	}
	for _, root := range []string{e.Root(), repair.TargetPath} {
		body, err := os.ReadFile(filepath.Join(root, "README.md"))
		if err != nil || !strings.Contains(string(body), "<<<<<<<") || !strings.Contains(string(body), "Second conflicting output") {
			t.Fatalf("repair did not start from conflict markers in %s: %v", root, err)
		}
		if git(t, root, "diff", "--name-only", "--diff-filter=U") != "README.md" {
			t.Fatalf("repair index has no unresolved merge in %s", root)
		}
	}
	if !strings.Contains(briefText(t, &repair), "Resolve only the unit merge conflicts") {
		t.Fatal("repair brief omitted the merge repair instructions")
	}
	repeated := nextResult(t, e, ctx)
	if len(repeated.Instructions) != 1 || repeated.Instructions[0].ID != repair.ID {
		t.Fatal("repeated next replaced the repair unit")
	}
	write(t, work.Instructions[2].TargetPath, "third.md", "Late unfixed output\n")
	finishUnit(t, e, repair, "README.md", "Resolved unit outputs\n")
	resumed, err := newTestEngine(ctx, e.Root(), e.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	judge := nextResult(t, resumed, ctx)
	if judge.Instruction == nil || judge.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("repair and remaining fixed input were not judged: %+v", judge)
	}
	if git(t, e.Root(), "rev-parse", "HEAD") != before || git(t, e.Root(), "diff", "--name-only", "--diff-filter=U") != "" {
		t.Fatal("repair moved HEAD or left conflicts unresolved")
	}
	for path, want := range map[string]string{"README.md": "Resolved unit outputs\n", "third.md": "Third fixed output\n"} {
		body, err := os.ReadFile(filepath.Join(e.Root(), path))
		if err != nil || string(body) != want {
			t.Fatalf("fixed unit result was lost: %s: %q, %v", path, body, err)
		}
	}
	state, err = resumed.load()
	if err != nil {
		t.Fatal(err)
	}
	merged, _ := state.RoundMerge(rounds[1].ID)
	if merged.Status != "merged" || merged.Applied != 3 || merged.Inputs[2] != original.Inputs[2] || len(state.UnitResults(rounds[1].ID)) != 1 {
		t.Fatalf("repair lost the original fixed inputs: %+v", merged)
	}
	if !strings.Contains(briefText(t, judge.Instruction), "original-unit-report-marker") {
		t.Fatal("judgment omitted the original unit report after repair")
	}
	data, err := resumed.readInstruction(judge.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data.Request.Progress == nil || len(data.Request.Progress.WorkPerformed) != 4 {
		t.Fatal("repair discarded the original unit progress")
	}
	write(t, repair.TargetPath, "README.md", "Late repair edit\n")
	collect(t, resumed, judge.Instruction, pass(*data.Request))
	if done := nextResult(t, resumed, ctx); done.Status != ledger.WorkCompleted {
		t.Fatalf("repaired stage did not complete: %+v", done)
	}
	state, err = resumed.load()
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := resumed.adoptedOutputs(state)
	if err != nil || len(outputs) != 1 || !strings.Contains(outputs[0].Content, "original-unit-report-marker") {
		t.Fatalf("adopted output omitted original unit reports: %v", err)
	}
	if git(t, e.Root(), "show", "HEAD:README.md") != "Resolved unit outputs" {
		t.Fatal("completion reread the repair worktree after fixing its result")
	}
}
