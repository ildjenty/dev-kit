package ledger

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/oklog/ulid/v2"
)

func TestDecisionStatusMatchesAnswer(t *testing.T) {
	for _, status := range []DecisionStatus{DecisionApproved, DecisionRejected, DecisionDeferred} {
		for _, interpretation := range []Interpretation{AnswerApproved, AnswerRejected, AnswerDeferred, AnswerText} {
			t.Run(string(status)+"/"+string(interpretation), func(t *testing.T) {
				store, _ := workStore(t)
				v := testValidator(t)
				appendRecord(t, store, &Decision{Decision: form.Decision{ID: "choice"}})
				target := form.ApprovalTarget{Kind: form.ApprovalDecision, DecisionID: "choice"}
				answer := appendRecord(t, store, &Answer{QuestionID: "review", Target: target, Interpretation: interpretation, Text: "Use the measured option"})
				record, err := NewRecord(store, &StateChange{DecisionID: "choice", DecisionStatus: status, AnswerID: answer.ID})
				if err != nil {
					t.Fatal(err)
				}
				valid := string(status) == string(interpretation)
				if err := Append(store, record, v); (err == nil) != valid || !valid && !strings.Contains(err.Error(), "does not match answer interpretation") {
					t.Fatalf("decision state write: %v", err)
				}
				if !valid {
					if err := store.AppendRecord(record.ID, string(record.Kind()), record.Payload); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := Get(store, record.ID, v); (err == nil) != valid || !valid && !strings.Contains(err.Error(), "does not match answer interpretation") {
					t.Fatalf("decision state read: %v", err)
				}
				if _, err := List(store, v); (err == nil) != valid || !valid && !strings.Contains(err.Error(), "does not match answer interpretation") {
					t.Fatalf("decision state scan: %v", err)
				}
				if valid {
					resolved, err := loadState(t, store).ResolveDecisions(DecisionContext{})
					if err != nil || len(resolved) != 1 || resolved[0].Status != status {
						t.Fatalf("decision status: %+v, %v", resolved, err)
					}
				}
			})
		}
	}
}

func TestJudgmentProvenanceValidation(t *testing.T) {
	v := testValidator(t)
	for _, tc := range []struct {
		name       string
		provenance check.Provenance
		valid      bool
	}{
		{"empty", check.Provenance{}, false},
		{"unknown source", check.Provenance{Source: "unknown", InstructionID: "judge", Runner: "judge"}, false},
		{"missing instruction", check.Provenance{Source: check.SubagentResult, Runner: "judge"}, false},
		{"missing runner", check.Provenance{Source: check.SubagentResult, InstructionID: "judge"}, false},
		{"different instruction", check.Provenance{Source: check.SubagentResult, InstructionID: "other", Runner: "judge"}, false},
		{"different runner", check.Provenance{Source: check.SubagentResult, InstructionID: "judge", Runner: "other"}, false},
		{"subagent", check.Provenance{Source: check.SubagentResult, InstructionID: "judge", Runner: "judge"}, true},
		{"judge command", check.Provenance{Source: check.JudgeCommand, InstructionID: "judge", Runner: "judge"}, true},
		{"command check", check.Provenance{Source: check.CommandCheck, InstructionID: "judge", Runner: "judge"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := fsstore.New(t.TempDir())
			judgment := &Judgment{Judgment: check.Judgment{JudgmentResult: form.JudgmentResult{Judgment: form.Pass}}}
			completeFixture(judgment)
			judgment.Provenance = tc.provenance
			record, err := NewRecord(store, judgment)
			if err != nil {
				t.Fatal(err)
			}
			if err := Append(store, record, v); (err == nil) != tc.valid || !tc.valid && !strings.Contains(err.Error(), "provenance") {
				t.Fatalf("judgment write: %v", err)
			}
			if !tc.valid {
				if err := store.AppendRecord(record.ID, string(record.Kind()), record.Payload); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Get(store, record.ID, v); (err == nil) != tc.valid || !tc.valid && !strings.Contains(err.Error(), "provenance") {
				t.Fatalf("judgment read: %v", err)
			}
			if _, err := List(store, v); (err == nil) != tc.valid || !tc.valid && !strings.Contains(err.Error(), "provenance") {
				t.Fatalf("judgment scan: %v", err)
			}
		})
	}
}

func TestGetRejectsMissingReferences(t *testing.T) {
	root := t.TempDir()
	store := fsstore.New(root)
	v := testValidator(t)
	missing := ulid.Make().String()
	evidence := &Evidence{Evidence: form.Evidence{ID: "source", Content: "Measured latency", VerificationMethod: "Benchmark", Sources: []form.Reference{{RecordID: missing}}}}
	completeFixture(evidence)
	record, err := NewRecord(store, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRecord(record.ID, string(record.Kind()), evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(store, record.ID, v); err == nil || !strings.Contains(err.Error(), record.ID+".json") || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing reference diagnostic: %v", err)
	}
	if _, err := List(store, v); err == nil {
		t.Fatal("missing reference accepted by List")
	}
	decision := &Decision{Decision: form.Decision{ID: "choice", Evidence: []form.Reference{{RecordID: record.ID}}}}
	completeFixture(decision)
	dependent, err := NewRecord(store, decision)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(store, dependent, v); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("reference to corrupt record accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "records", record.ID+".json")); err != nil {
		t.Fatal(err)
	}
}

func TestPayloadValidationAtStorageBoundaries(t *testing.T) {
	v := testValidator(t)
	for _, payload := range []Payload{
		&Work{}, &Pause{}, &Resume{}, &PlanVersion{}, &Withdrawal{}, &Decision{},
		&Evidence{}, &Lesson{}, &StateChange{}, &Question{}, &Answer{}, &Approval{},
		&ExternalOp{}, &Progress{}, &Judgment{}, &StageRun{},
		&Round{}, &Instruction{}, &Artifact{},
	} {
		t.Run(string(payload.recordKind()), func(t *testing.T) {
			root := t.TempDir()
			store := fsstore.New(root)
			record, err := NewRecord(store, payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := Append(store, record, v); err == nil || !strings.Contains(err.Error(), record.ID+".json") {
				t.Fatalf("invalid payload write diagnostic: %v", err)
			}
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.AppendRecord(record.ID, string(record.Kind()), json.RawMessage(data)); err != nil {
				t.Fatal(err)
			}
			if _, err := Get(store, record.ID, v); err == nil || !strings.Contains(err.Error(), record.ID+".json") {
				t.Fatalf("invalid payload read diagnostic: %v", err)
			}
			if _, err := List(store, v); err == nil {
				t.Fatal("invalid payload accepted by List")
			}
			if _, err := os.Stat(filepath.Join(root, "records", record.ID+".json")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRawPayloadRequiredFieldsAndConstraints(t *testing.T) {
	v := testValidator(t)
	for _, tc := range []struct {
		kind   Kind
		body   string
		reason string
	}{
		{KindDecision, `{}`, "missing properties"},
		{KindProgress, `{"round_id":"round","instruction_id":"work","termination_kind":"invalid"}`, "termination_kind"},
		{KindAnswer, `{"question_id":"q","target":{"kind":"plan","plan":1},"interpretation":"approved","text":"","original":"Approved","session_id":"session"}`, "observed_at"},
		{KindPlan, `{"plan":{"version":1},"origin":"human"}`, "missing properties"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			store := fsstore.New(t.TempDir())
			id := ulid.Make().String()
			if err := store.AppendRecord(id, string(tc.kind), json.RawMessage(tc.body)); err != nil {
				t.Fatal(err)
			}
			if _, err := Get(store, id, v); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("invalid raw payload diagnostic: %v", err)
			}
		})
	}
}

func TestWithdrawnHumanPlanPreservesOldInstructions(t *testing.T) {
	for _, origin := range []plan.ProposalOrigin{plan.HumanProposal, plan.BackProposal} {
		t.Run(string(origin), func(t *testing.T) {
			store, _ := workStore(t)
			appendRecord(t, store, &PlanVersion{Plan: completePlanFixture(plan.Plan{Version: 1}), Origin: plan.InitialProposal})
			original := &Instruction{ID: "plan-1-check", Kind: form.InstructionPlanCheck, TargetKey: "plan:1", SessionID: "session", PlanVersion: 1}
			appendRecord(t, store, original)
			appendRecord(t, store, &PlanVersion{Plan: completePlanFixture(plan.Plan{Version: 2}), Origin: origin})
			appendRecord(t, store, &Instruction{ID: "plan-2-check", Kind: form.InstructionPlanCheck, TargetKey: "plan:2", SessionID: "session", PlanVersion: 2})
			if pending := loadState(t, store).UnacceptedInstructions(); len(pending) != 1 || pending[0].ID != "plan-2-check" {
				t.Fatalf("pending plan: %+v", pending)
			}
			appendRecord(t, store, &Withdrawal{PlanVersion: 2})
			check := func(want *Instruction) {
				t.Helper()
				pending := loadState(t, store).UnacceptedInstructions()
				if origin != plan.HumanProposal {
					if len(pending) != 0 {
						t.Fatalf("back proposal restored old instruction: %+v", pending)
					}
				} else if len(pending) != 1 || !reflect.DeepEqual(pending[0], *want) {
					t.Fatalf("old instruction not restored: %+v", pending)
				}
			}
			check(original)
			reissued := *original
			reissued.ID, reissued.Reissue = "plan-1-check-again", 1
			appendRecord(t, store, &reissued)
			check(&reissued)
		})
	}
}

func TestRecordIDsAcrossProcesses(t *testing.T) {
	const seedID = "7ZZZZZZZZZZZZZZZZZZZZZZZZ0"
	if root := os.Getenv("DEVKIT_LEDGER_ORDER_TEST_ROOT"); root != "" {
		store := fsstore.New(root)
		lock, err := store.Lock()
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		state := loadState(t, store)
		record, err := NewRecord(store, &StageRun{ID: seedID, StageID: "implement", PlanVersion: 1, State: StageWorking, StopReason: os.Getenv("DEVKIT_LEDGER_ORDER_TEST_WRITER")})
		if err != nil {
			t.Fatal(err)
		}
		if record.ID <= state.LastRecordID() {
			t.Fatalf("ID %s does not follow %s", record.ID, state.LastRecordID())
		}
		if err := Append(store, record, testValidator(t)); err != nil {
			t.Fatal(err)
		}
		if got := loadState(t, store).LastRecordID(); got != record.ID {
			t.Fatalf("last record: got %s, want %s", got, record.ID)
		}
		return
	}
	root := t.TempDir()
	store := fsstore.New(root)
	seed := Record{ID: seedID, Payload: &StageRun{ID: seedID, StageID: "implement", PlanVersion: 1, State: StageWorking}}
	if err := Append(store, seed, testValidator(t)); err != nil {
		t.Fatal(err)
	}
	commands := []*exec.Cmd{}
	for _, writer := range []string{"first", "second", "third"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRecordIDsAcrossProcesses$")
		cmd.Env = append(os.Environ(), "DEVKIT_LEDGER_ORDER_TEST_ROOT="+root, "DEVKIT_LEDGER_ORDER_TEST_WRITER="+writer)
		commands = append(commands, cmd)
	}
	results := make(chan error, len(commands))
	for _, cmd := range commands {
		go func() {
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("child process: %s", output)
			}
			results <- err
		}()
	}
	for range commands {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	state := loadState(t, store)
	if len(state.records) != 4 || state.LastRecordID() <= seedID {
		t.Fatalf("records: %+v", state.records)
	}
	last := state.records[len(state.records)-1]
	runs := state.StageRuns()
	if len(runs) != 1 || !reflect.DeepEqual(runs[0], last.Payload) {
		t.Fatalf("latest stage state: %+v, last record: %+v", runs, last)
	}
}
