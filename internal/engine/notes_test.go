package engine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func workerTestNotes() []form.Note {
	return []form.Note{
		{Kind: "decision", Payload: json.RawMessage(`{"id":"D1","content":"Keep the existing interface","validity":"During this task","supersedes":[],"depends_on":[],"conditions":[],"decider":"worker","provisional":true,"rejected_alternatives":[],"evidence":[]}`)},
		{Kind: "evidence", Payload: json.RawMessage(`{"id":"E1","content":"The existing interface covers the task","sources":[{"external":"README.md"}],"verification_method":"Read the documented interface"}`)},
		{Kind: "lesson", Payload: json.RawMessage(`{"content":"Check the documented interface before changing it","evidence":[{"external":"README.md"}]}`)},
	}
}

func assertWorkerNotes(t *testing.T, e *Engine, count int) {
	t.Helper()
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[ledger.Kind]int{}
	keys := map[string]bool{}
	for _, record := range records {
		switch record.Kind() {
		case ledger.KindDecision, ledger.KindEvidence, ledger.KindLesson:
			counts[record.Kind()]++
			if record.NoteKey == "" || keys[record.NoteKey] {
				t.Fatalf("missing or duplicate worker note key: %+v", record)
			}
			keys[record.NoteKey] = true
		}
	}
	for _, kind := range []ledger.Kind{ledger.KindDecision, ledger.KindEvidence, ledger.KindLesson} {
		if counts[kind] != count {
			t.Fatalf("%s records: got %d, want %d", kind, counts[kind], count)
		}
	}
}

func TestWorkerNotesAcceptedOnce(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "whole-stage"
		if split {
			name = "split-unit"
		}
		t.Run(name, func(t *testing.T) {
			var e *Engine
			var work Result
			if split {
				e, _, work = splitWork(t, 3)
			} else {
				e, work = roleWork(t, func(stage *plan.Stage) {
					stage.Assignee = "agent"
					stage.Checks = []plan.Check{judgeCheck("wording", true)}
				})
			}
			for _, instruction := range work.Instructions {
				collect(t, e, &instruction, form.Progress{InstructionID: instruction.ID, TerminationKind: form.TerminationCompleted, Notes: workerTestNotes()})
			}
			resumed, err := newTestEngine(t.Context(), e.Root(), e.sessionID)
			if err != nil {
				t.Fatal(err)
			}
			judgment := nextResult(t, resumed, t.Context())
			if judgment.Instruction == nil || judgment.Instruction.Kind != form.InstructionJudgment {
				t.Fatalf("worker results did not reach judgment: %+v", judgment)
			}
			assertWorkerNotes(t, resumed, len(work.Instructions))
			for range 2 {
				nextResult(t, resumed, t.Context())
			}
			assertWorkerNotes(t, resumed, len(work.Instructions))
			state, err := resumed.load()
			if err != nil {
				t.Fatal(err)
			}
			// Re-reading and directly re-accepting the hook result must also be harmless.
			records, err := resumed.records()
			if err != nil {
				t.Fatal(err)
			}
			for _, record := range records {
				instruction, ok := record.Payload.(*ledger.Instruction)
				if !ok || instruction.Kind != form.InstructionUnitWork {
					continue
				}
				progress := form.Progress{InstructionID: instruction.ID, TerminationKind: form.TerminationCompleted, Notes: workerTestNotes()}
				obs := observedResult{path: "results/repeated.json"}
				obs.result.AgentID = "worker"
				obs.result.Content = marshal(t, progress)
				if _, err := resumed.acceptUnitResult(state, *instruction, obs); err != nil {
					t.Fatal(err)
				}
			}
			assertWorkerNotes(t, resumed, len(work.Instructions))
		})
	}
}

func TestInvalidWorkerNoteReissuesWithReason(t *testing.T) {
	for _, tc := range []struct {
		name, kind, payload, reason string
	}{
		{"missing content", "lesson", `{"evidence":[]}`, "content"},
		{"unknown field", "lesson", `{"content":"Lesson","evidence":[],"unexpected":true}`, "unexpected"},
		{"unknown kind", "other", `{}`, "kind"},
		{"missing reference", "lesson", `{"content":"Lesson","evidence":[{"record_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV"}]}`, "reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _, work := splitWork(t, 1)
			instruction := work.Instructions[0]
			notes := append(workerTestNotes(), form.Note{Kind: tc.kind, Payload: json.RawMessage(tc.payload)})
			collect(t, e, &instruction, form.Progress{InstructionID: instruction.ID, TerminationKind: form.TerminationCompleted, Notes: notes})
			retry := nextResult(t, e, t.Context())
			if len(retry.Instructions) != 1 || retry.Instructions[0].ID == instruction.ID || retry.Instructions[0].Reissue != 1 {
				t.Fatalf("invalid note did not reissue the worker: %+v", retry)
			}
			if !strings.Contains(briefText(t, &retry.Instructions[0]), tc.reason) {
				t.Fatalf("reissue omitted rejection reason %q", tc.reason)
			}
			assertWorkerNotes(t, e, 0)
			state, err := e.load()
			if err != nil {
				t.Fatal(err)
			}
			round, _ := state.OpenRound()
			if len(state.UnitResults(round.ID)) != 0 {
				t.Fatal("invalid notes left an accepted unit result")
			}
			fixed := retry.Instructions[0]
			collect(t, e, &fixed, form.Progress{InstructionID: fixed.ID, TerminationKind: form.TerminationCompleted, Notes: workerTestNotes()})
			nextResult(t, e, t.Context())
			assertWorkerNotes(t, e, 1)
		})
	}
}

func TestWorkerNotesResumePartialPublicationAfterReissue(t *testing.T) {
	e, _, work := splitWork(t, 1)
	instruction := work.Instructions[0]
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	meta := state.UnacceptedInstructions()[0]
	notes, err := e.workerNotes(meta.TargetKey, workerTestNotes())
	if err != nil {
		t.Fatal(err)
	}
	// Simulate interruption after one note was published but before the result was accepted.
	if err := ledger.Append(e.store(), notes[0], e.validator); err != nil {
		t.Fatal(err)
	}
	if err := hook.PublishSession(e.runRoot(), hook.SessionObservation{Host: event.ClaudeCode, SessionID: "reissued-notes", ObservedAt: time.Now().UTC()}, "SessionStart"); err != nil {
		t.Fatal(err)
	}
	e, err = newTestEngine(t.Context(), e.Root(), "reissued-notes")
	if err != nil {
		t.Fatal(err)
	}
	retry := nextResult(t, e, t.Context())
	if len(retry.Instructions) != 1 || retry.Instructions[0].Reissue != 1 || retry.Instructions[0].ID == instruction.ID {
		t.Fatalf("new session did not reissue the worker: %+v", retry)
	}
	fixed := retry.Instructions[0]
	collect(t, e, &fixed, form.Progress{InstructionID: fixed.ID, TerminationKind: form.TerminationCompleted, Notes: workerTestNotes()})
	nextResult(t, e, t.Context())
	assertWorkerNotes(t, e, 1)
}

func TestReissuedWorkerWithDifferentNotesRecordsThem(t *testing.T) {
	e, _, work := splitWork(t, 1)
	instruction := work.Instructions[0]
	bad := append(workerTestNotes(), form.Note{Kind: "lesson", Payload: json.RawMessage(`{"evidence":[]}`)})
	collect(t, e, &instruction, form.Progress{InstructionID: instruction.ID, TerminationKind: form.TerminationCompleted, Notes: bad})
	retry := nextResult(t, e, t.Context())
	if len(retry.Instructions) != 1 || retry.Instructions[0].Reissue != 1 {
		t.Fatalf("invalid note did not reissue the worker: %+v", retry)
	}
	// The reissued worker rewrites its lesson; the new content is recorded, not refused.
	changed := workerTestNotes()
	changed[2] = form.Note{Kind: "lesson", Payload: json.RawMessage(`{"content":"Read the interface documentation first","evidence":[]}`)}
	fixed := retry.Instructions[0]
	collect(t, e, &fixed, form.Progress{InstructionID: fixed.ID, TerminationKind: form.TerminationCompleted, Notes: changed})
	nextResult(t, e, t.Context())
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	if len(state.UnitResults(round.ID)) != 1 {
		t.Fatal("reissued worker with different notes was not accepted")
	}
	assertWorkerNotes(t, e, 1)
}

func TestSubmittedNotesAreRecordedOnce(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Updated\n")
	progress := form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated README"}, Notes: workerTestNotes()}
	for range 2 {
		if _, err := e.Submit(ctx, marshal(t, progress)); err != nil {
			t.Fatal(err)
		}
	}
	assertWorkerNotes(t, e, 1)
}
