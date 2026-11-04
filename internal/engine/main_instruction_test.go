package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestMainInstructionsReachOnlyMatchingPlannerAndWorkerBriefs(t *testing.T) {
	for _, session := range []bool{false, true} {
		name := "subagent"
		if session {
			name = "session"
		}
		t.Run(name, func(t *testing.T) {
			factory := defaultFixture
			if session {
				factory = fixture
			}
			e, ctx := factory(t)
			configureCatalogEntry(t, e, "wording", "history = 'all'")
			configureStage(t, e, func(stage *plan.Stage) {
				stage.Checks = []plan.Check{judgeCheck("wording", true)}
				stage.History = plan.HistoryAll
			})
			start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Keep the main task intact"})
			if err != nil {
				t.Fatal(err)
			}
			original := briefText(t, start.Instruction)
			instructions := []ledger.MainInstruction{
				{Text: "  Preserve this wording verbatim.\n{{literal_template}}\n"},
				{Text: "worker-only-marker", Audience: "worker", StageID: "implement"},
				{Text: "planner-only-marker", Audience: "planner"},
				{Text: "other-stage-marker", StageID: "scope"},
				{Text: "Change the completion conditions only through a new plan."},
			}
			for _, instruction := range instructions {
				if _, err := e.Instruct(ctx, marshal(t, instruction)); err != nil {
					t.Fatal(err)
				}
			}
			if got := briefText(t, start.Instruction); got != original {
				t.Fatal("a new instruction changed the outstanding planner brief")
			}
			planResult(t, e, start)
			review := nextResult(t, e, ctx)
			if review.Instruction == nil || review.Instruction.Kind != form.InstructionPlanCheck {
				t.Fatalf("expected plan check: %+v", review)
			}
			assertNoMainInstructions(t, briefText(t, review.Instruction), instructions)
			data, err := e.readInstruction(review.Instruction.ID)
			if err != nil {
				t.Fatal(err)
			}
			collect(t, e, review.Instruction, pass(*data.Request))
			work := roleNext(t, e)
			worker := work.Instruction
			if !session {
				worker = &work.Instructions[0]
			}
			body := briefText(t, worker)
			for _, instruction := range []ledger.MainInstruction{instructions[0], instructions[1], instructions[4]} {
				if !strings.Contains(body, instruction.Text) {
					t.Fatalf("worker omitted verbatim instruction %q", instruction.Text)
				}
			}
			for _, marker := range []string{"planner-only-marker", "other-stage-marker"} {
				if strings.Contains(body, marker) {
					t.Fatalf("worker received mismatched instruction %q", marker)
				}
			}
			history, err := os.ReadFile(filepath.Join(e.workRoot(), "brief", worker.ID+"-history.md"))
			if err != nil || !bytes.Contains(history, []byte(instructions[0].Text)) {
				t.Fatalf("worker history omitted the instruction record: %v", err)
			}
			if _, err := e.Instruct(ctx, []byte(`{"text":"later-instruction-marker"}`)); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(worker.Brief); err != nil {
				t.Fatal(err)
			}
			restored, err := e.instructionBrief(worker.ID)
			if err != nil || string(restored) != body {
				t.Fatalf("regeneration changed the issued worker brief: %v", err)
			}
			if session {
				submitWork(t, e, work)
			} else {
				collect(t, e, worker, form.Progress{InstructionID: worker.ID, TerminationKind: form.TerminationCompleted})
			}
			judge := nextResult(t, e, ctx)
			if judge.Instruction == nil || judge.Instruction.Kind != form.InstructionJudgment {
				t.Fatalf("expected judgment: %+v", judge)
			}
			assertNoMainInstructions(t, briefText(t, judge.Instruction), instructions)
			judgeHistory, err := os.ReadFile(filepath.Join(e.workRoot(), "brief", judge.Instruction.ID+"-history.md"))
			if err != nil {
				t.Fatal(err)
			}
			assertNoMainInstructions(t, string(judgeHistory), instructions)
			planner, err := e.planner(ctx, "test", 2, plan.HumanProposal, "")
			if err != nil {
				t.Fatal(err)
			}
			planned := briefText(t, planner.Instruction)
			for _, instruction := range []ledger.MainInstruction{instructions[0], instructions[2], instructions[3], instructions[4]} {
				if !strings.Contains(planned, instruction.Text) {
					t.Fatalf("planner omitted verbatim instruction %q", instruction.Text)
				}
			}
			if strings.Contains(planned, "worker-only-marker") || !strings.Contains(planned, "Target stage: scope") {
				t.Fatal("planner instruction audience or target was lost")
			}
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			p, _ := latestPlan(state)
			if p.Plan.Version != 1 || p.Plan.CompletionConditions[0].Description != "The documentation is updated" {
				t.Fatal("an instruction changed the plan")
			}
		})
	}
}

func assertNoMainInstructions(t *testing.T, body string, instructions []ledger.MainInstruction) {
	t.Helper()
	for _, instruction := range instructions {
		if strings.Contains(body, instruction.Text) {
			t.Fatalf("judge request received main instruction %q", instruction.Text)
		}
	}
}

func TestInstructRejectsInvalidInputAndInactiveWork(t *testing.T) {
	e, ctx := fixture(t)
	if _, err := e.Instruct(ctx, []byte(`{"text":"No work yet"}`)); err == nil {
		t.Fatal("accepted an instruction without active work")
	}
	if _, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Validate instructions"}); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{}`, `{"text":""}`, `{"text":"   "}`, `{"text":"x","stage_id":""}`, `{"text":"x","audience":"judge"}`, `{"text":"x","audience":"both"}`, `{"text":"x","extra":true}`} {
		if _, err := e.Instruct(ctx, []byte(data)); err == nil {
			t.Fatalf("accepted invalid instruction: %s", data)
		}
	}
	if _, err := e.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Instruct(ctx, []byte(`{"text":"Work has ended"}`)); err == nil {
		t.Fatal("accepted an instruction after work ended")
	}
}
