package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func TestInstructionFactsSurviveDeletedBriefs(t *testing.T) {
	e, ctx := fixture(t)
	removeBriefs := func() {
		t.Helper()
		if err := os.RemoveAll(filepath.Join(e.workRoot(), "brief")); err != nil {
			t.Fatal(err)
		}
	}
	started, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	removeBriefs()
	if runner, err := e.InstructionRunner(ledger.Instruction{ID: started.Instruction.ID}); err != nil || runner != "devkit-planner" {
		t.Fatalf("runner depended on a generated file: %q, %v", runner, err)
	}
	planResult(t, e, started)
	detection, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if instruction, ok := record.Payload.(*ledger.Instruction); ok && instruction.Request != nil {
			for _, norm := range instruction.Request.Norms {
				if norm.Content != "" {
					t.Fatal("instruction record stored rendered norm content")
				}
			}
		}
	}
	original, err := os.ReadFile(detection.Instruction.Brief)
	if err != nil {
		t.Fatal(err)
	}
	removeBriefs()
	repeated, err := e.Next(ctx)
	if err != nil || repeated.Instruction == nil || repeated.Instruction.ID != detection.Instruction.ID {
		t.Fatalf("detection instruction could not be rebuilt: %+v, %v", repeated, err)
	}
	body, err := os.ReadFile(repeated.Instruction.Brief)
	if err != nil || string(body) != string(original) {
		t.Fatalf("rebuilt detection brief changed: %v", err)
	}
	removeBriefs()
	acceptDetectionResult(t, e, detection.Instruction)
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	removeBriefs()
	collect(t, e, review.Instruction, pass(*data.Request))
	work, err := e.Next(ctx)
	work = approvedNext(t, e, work)
	if err != nil || work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("plan result depended on a generated file: %+v, %v", work, err)
	}
	original, err = os.ReadFile(work.Instruction.Brief)
	if err != nil {
		t.Fatal(err)
	}
	removeBriefs()
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	body, err = e.instructionBrief(work.Instruction.ID)
	if err != nil || string(body) != string(original) {
		t.Fatalf("accepted work brief could not be rebuilt: %v", err)
	}
}
