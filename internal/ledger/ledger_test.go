package ledger

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
	"github.com/oklog/ulid/v2"
)

func appendRecord(t *testing.T, store *fsstore.Store, payload Payload) Record {
	t.Helper()
	completeFixture(payload)
	if _, pause := payload.(*Pause); !pause {
		lock, err := store.Lock()
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
	}
	record, err := NewRecord(store, payload)
	if err != nil {
		t.Fatal(err)
	}
	switch p := payload.(type) {
	case *StageRun:
		if p.ID == "" {
			p.ID = record.ID
		}
	case *Round:
		if p.ID == "" {
			p.ID = record.ID
		}
	}
	if err := Append(store, record, testValidator(t)); err != nil {
		t.Fatal(err)
	}
	return record
}

func workStore(t *testing.T) (*fsstore.Store, *Work) {
	t.Helper()
	store := fsstore.New(t.TempDir())
	id := ulid.Make().String()
	work := &Work{ID: id, TaskRef: "issue:42", Branch: "devkit/" + id, BaseCommit: "base", KitVersion: ReadableKitVersion}
	appendRecord(t, store, work)
	return store, work
}

func loadState(t *testing.T, store *fsstore.Store, stops ...StopObservation) *State {
	t.Helper()
	state, err := Load(store, stops, testValidator(t))
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRecordStorage(t *testing.T) {
	store, work := workStore(t)
	evidence := appendRecord(t, store, &Evidence{Evidence: form.Evidence{ID: "source", Content: "Measured latency", VerificationMethod: "benchmark"}})
	decision := appendRecord(t, store, &Decision{Decision: form.Decision{ID: "choice", Content: "Use the measured option", Evidence: []form.Reference{{RecordID: evidence.ID}}}})
	got, err := Get(store, decision.ID, testValidator(t))
	if err != nil || got.Kind() != KindDecision || got.Payload.(*Decision).Evidence[0].RecordID != evidence.ID {
		t.Fatalf("typed record: %+v, %v", got, err)
	}
	if err := Append(store, decision, testValidator(t)); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("duplicate append: %v", err)
	}
	missing, err := NewRecord(store, &Evidence{Evidence: form.Evidence{ID: "missing", Content: "Missing source", VerificationMethod: "Inspection", Sources: []form.Reference{{RecordID: ulid.Make().String()}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(store, missing, testValidator(t)); err == nil {
		t.Fatal("missing reference accepted")
	}
	for _, data := range []string{
		`{"id":"` + decision.ID + `","kind":"future","payload":{}}`,
		`{"id":"` + decision.ID + `","kind":"decision","payload":null}`,
		`{"id":"` + decision.ID + `","kind":"decision","payload":{"unknown":true}}`,
	} {
		raw, err := fsstore.DecodeRecord(decision.ID, []byte(data))
		if err == nil {
			_, err = readRecord(raw, testValidator(t))
		}
		if err == nil {
			t.Fatalf("unreadable record accepted: %s", data)
		}
	}
	incompatibleWork := *work
	incompatibleWork.KitVersion = "future"
	incompatible, err := NewRecord(store, &incompatibleWork)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(store, incompatible, testValidator(t)); err == nil || !strings.Contains(err.Error(), "unreadable kit version") {
		t.Fatalf("unknown kit diagnostic: %v", err)
	}
	var typedNil *Work
	nilRecord, err := NewRecord(store, typedNil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(store, nilRecord, testValidator(t)); err == nil {
		t.Fatal("nil payload accepted")
	}
}

func TestCorruptRecordRemainsInPlace(t *testing.T) {
	root := t.TempDir()
	store := fsstore.New(root)
	id := ulid.Make().String()
	data := json.RawMessage(`{"id":"` + id + `","kind":"future","payload":{}}`)
	if err := store.Append(id, data); err != nil {
		t.Fatal(err)
	}
	if _, err := List(store, testValidator(t)); err == nil || !strings.Contains(err.Error(), id+".json") || !strings.Contains(err.Error(), "unknown record kind") {
		t.Fatalf("corrupt record diagnostic: %v", err)
	}
	if contents, err := os.ReadFile(filepath.Join(root, "records", id+".json")); err != nil || string(contents) != string(data) {
		t.Fatalf("record changed: %s, %v", contents, err)
	}
}

func TestCheckpointPauseAndStopQuestions(t *testing.T) {
	store, work := workStore(t)
	run := &StageRun{StageID: "implement", PlanVersion: 1, State: StageWorking}
	appendRecord(t, store, run)
	round := &Round{StageRunID: run.ID, SessionID: "session-a", StartTarget: form.TargetVersion{Kind: form.TargetTree, Tree: "before"}}
	appendRecord(t, store, round)
	for count, session := range []string{"session-a", "session-b"} {
		appendRecord(t, store, &Instruction{ID: "work-" + session, Kind: form.InstructionWork, TargetKey: round.ID, Reissue: count, SessionID: session, RoundID: round.ID})
	}
	if pending := loadState(t, store).UnacceptedInstructions(); len(pending) != 1 || pending[0].ID != "work-session-b" {
		t.Fatalf("reissued instruction: %+v", pending)
	}
	appendRecord(t, store, &Progress{RoundID: round.ID, Progress: form.Progress{
		InstructionID: "work-session-b", TerminationKind: form.TerminationCompleted,
		TemporaryState: []form.TemporaryState{{Content: "Current cache", InvalidWhen: "Cache was replaced"}, {Content: "Old cache", InvalidWhen: "Cache was replaced"}},
	}})
	lock, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	pause := appendRecord(t, store, &Pause{Position: form.Reference{RecordID: round.ID}})
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	appendRecord(t, store, &Judgment{RoundID: round.ID, Judgment: check.Judgment{JudgmentResult: form.JudgmentResult{InstructionID: "judge", Judgment: form.Pass}}})
	question := form.Question{ID: "continue", Text: "Continue work?", ApprovalTarget: form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: work.ID}}
	stop := StopObservation{ID: ulid.Make().String(), WorkID: work.ID, LimitReached: true, Question: &question}
	state := loadState(t, store, stop)
	if _, id, paused := state.Paused(); !paused || id != pause.ID {
		t.Fatalf("result arrival cleared pause: %s, %v", id, paused)
	}
	if current, open := state.OpenRound(); !open || current.ID != round.ID {
		t.Fatalf("pause invalidated round: %+v, %v", current, open)
	}
	if _, err := state.Checkpoint(nil); err == nil {
		t.Fatal("temporary state was used without checking validity")
	}
	before, _ := state.LatestProgress()
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := state.Checkpoint(func(item form.TemporaryState) (bool, error) { return item.Content == "Current cache", nil })
	if err != nil || !checkpoint.Position.Paused || len(checkpoint.Questions) != 1 || len(checkpoint.Instructions) != 0 || len(checkpoint.Progress.TemporaryState) != 1 || checkpoint.Progress.TemporaryState[0].Content != "Current cache" {
		t.Fatalf("checkpoint: %+v, %v", checkpoint, err)
	}
	progress, _ := state.LatestProgress()
	afterJSON, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeJSON, afterJSON) {
		t.Fatal("checkpoint mutated stored progress")
	}
	appendRecord(t, store, &Question{Question: checkpoint.Questions[0]})
	if questions := loadState(t, store, stop).UnansweredQuestions(); len(questions) != 1 {
		t.Fatalf("incorporated question duplicated: %+v", questions)
	}
	appendRecord(t, store, &Answer{QuestionID: question.ID, Target: question.ApprovalTarget, Interpretation: AnswerText, Text: "Continue", Original: "Please continue", SessionID: "session-b", ObservedAt: time.Now()})
	if questions := loadState(t, store, stop).UnansweredQuestions(); len(questions) != 0 {
		t.Fatalf("answered stop question is still pending: %+v", questions)
	}
	newPause := appendRecord(t, store, &Pause{Position: form.Reference{RecordID: round.ID}})
	appendRecord(t, store, &Resume{PauseID: pause.ID})
	if _, id, paused := loadState(t, store).Paused(); !paused || id != newPause.ID {
		t.Fatal("resume cleared a different pause")
	}
	appendRecord(t, store, &Resume{PauseID: newPause.ID})
	if _, _, paused := loadState(t, store).Paused(); paused {
		t.Fatal("resume did not clear pause")
	}
}

func TestApprovalCorrectionsAndQuestionReplacement(t *testing.T) {
	store, _ := workStore(t)
	target := form.ApprovalTarget{Kind: form.ApprovalTree, Tree: "tree-a"}
	appendRecord(t, store, &Question{Question: form.Question{ID: "review-a", ApprovalTarget: target}})
	first := appendRecord(t, store, &Approval{Answer: Answer{QuestionID: "review-a", Target: target, Interpretation: AnswerApproved, Original: "Approved"}})
	if _, ok := loadState(t, store).ResolveApproval(target); !ok {
		t.Fatal("approval not resolved")
	}
	appendRecord(t, store, &Answer{QuestionID: "review-a", Target: target, Interpretation: AnswerRejected, Original: "I withdraw approval", Supersedes: first.ID})
	if _, ok := loadState(t, store).ResolveApproval(target); ok {
		t.Fatal("corrected approval remained effective")
	}
	changed := form.ApprovalTarget{Kind: form.ApprovalTree, Tree: "tree-b"}
	appendRecord(t, store, &Question{Question: form.Question{ID: "review-b", ApprovalTarget: changed}, Supersedes: "review-a"})
	state := loadState(t, store)
	if pending := state.UnansweredQuestions(); len(pending) != 1 || pending[0].ID != "review-b" {
		t.Fatalf("replacement question: %+v", pending)
	}
	if _, ok := state.ResolveAnswer(target); ok {
		t.Fatal("superseded question remained current")
	}
}

func TestPlanAndJudgmentQueries(t *testing.T) {
	store, _ := workStore(t)
	stage := plan.Stage{ID: "implement", Transitions: plan.Transitions{Fix: "implement", Ask: "implement", Back: "implement"}}
	template := plan.PlanTemplate{Name: "simple", StartStage: stage.ID, Stages: []plan.Stage{stage}}
	proposal := plan.NewPlan(template, "template-hash", "Implement the task", nil)
	appendRecord(t, store, &PlanVersion{Plan: proposal, Origin: plan.InitialProposal})
	norms := []form.NormReference{{Path: "rules", BlobID: "fixed"}}
	pass := func(version int) {
		t.Helper()
		appendRecord(t, store, &Judgment{Judgment: check.Judgment{JudgmentResult: form.JudgmentResult{
			InstructionID: "plan-check", Aspect: "plan", TargetVersion: form.TargetVersion{Kind: form.TargetPlan, Plan: version}, Norms: norms, Judgment: form.Pass,
		}, Runner: "judge"}})
	}
	pass(1)
	if executable, ok := loadState(t, store).ExecutablePlan(template, norms); !ok || executable.Version != 1 {
		t.Fatalf("initial plan: %+v, %v", executable, ok)
	}
	run := &StageRun{StageID: stage.ID, PlanVersion: 1, State: StageWorking}
	appendRecord(t, store, run)
	round := &Round{StageRunID: run.ID, SessionID: "session"}
	appendRecord(t, store, round)
	proposal.Version = 2
	appendRecord(t, store, &PlanVersion{Plan: proposal, Origin: plan.HumanProposal})
	if _, ok := loadState(t, store).ExecutablePlan(template, norms); ok {
		t.Fatal("pending proposal allowed old plan execution")
	}
	if position := loadState(t, store).CurrentPosition(); position.Plan.Plan.Version != 1 {
		t.Fatal("proposal changed the running stage binding")
	}
	appendRecord(t, store, &Withdrawal{PlanVersion: 2})
	if executable, ok := loadState(t, store).ExecutablePlan(template, norms); !ok || executable.Version != 1 {
		t.Fatal("voluntary withdrawal blocked old plan")
	}
	closed := *round
	closed.Decision = &router.Decision{Action: plan.Back}
	appendRecord(t, store, &closed)
	if _, ok := loadState(t, store).OpenRound(); ok {
		t.Fatal("decided round remained open")
	}
	if _, ok := loadState(t, store).ExecutablePlan(template, norms); ok {
		t.Fatal("back transition allowed old plan")
	}
	proposal.Version = 3
	appendRecord(t, store, &PlanVersion{Plan: proposal, Origin: plan.BackProposal})
	pass(3)
	if _, ok := loadState(t, store).ExecutablePlan(template, norms); ok {
		t.Fatal("replanning proceeded without approval")
	}
	target := form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: 3}
	appendRecord(t, store, &Question{Question: form.Question{ID: "plan-3", ApprovalTarget: target}})
	appendRecord(t, store, &Approval{Answer: Answer{QuestionID: "plan-3", Target: target, Interpretation: AnswerApproved}})
	if executable, ok := loadState(t, store).ExecutablePlan(template, norms); !ok || executable.Version != 3 {
		t.Fatalf("approved replan: %+v, %v", executable, ok)
	}
	changedNorms := []form.NormReference{{Path: "rules", BlobID: "changed"}}
	if _, ok := loadState(t, store).ExecutablePlan(template, changedNorms); ok {
		t.Fatal("changed norms did not invalidate judgment")
	}
}

func TestDecisionGraphAndAnswerStatus(t *testing.T) {
	store, _ := workStore(t)
	old := appendRecord(t, store, &Decision{Decision: form.Decision{ID: "old", Content: "Old choice"}})
	appendRecord(t, store, &Decision{Decision: form.Decision{ID: "dependent", DependsOn: []form.Reference{{RecordID: old.ID}}}})
	next := appendRecord(t, store, &Decision{Decision: form.Decision{ID: "next", Supersedes: []form.Reference{{RecordID: old.ID}}}})
	appendRecord(t, store, &Decision{Decision: form.Decision{ID: "latest", Supersedes: []form.Reference{{RecordID: next.ID}}}})
	resolved, err := loadState(t, store).ResolveDecisions(DecisionContext{Valid: func(form.Decision) (bool, error) { return true, nil }})
	if err != nil || len(resolved) != 4 || resolved[0].Valid || resolved[1].Valid || resolved[2].Valid || !resolved[3].Valid || len(resolved[1].InvalidDependencies) != 1 {
		t.Fatalf("replacement chain and dependency: %+v, %v", resolved, err)
	}
	target := form.ApprovalTarget{Kind: form.ApprovalDecision, DecisionID: "latest"}
	appendRecord(t, store, &Question{Question: form.Question{ID: "decision-review", ApprovalTarget: target}})
	approved := appendRecord(t, store, &Approval{Answer: Answer{QuestionID: "decision-review", Target: target, Interpretation: AnswerApproved}})
	appendRecord(t, store, &StateChange{DecisionID: "latest", DecisionStatus: DecisionApproved, AnswerID: approved.ID})
	resolved, err = loadState(t, store).ResolveDecisions(DecisionContext{Valid: func(form.Decision) (bool, error) { return true, nil }})
	if err != nil || resolved[3].Status != DecisionApproved {
		t.Fatalf("decision approval: %+v, %v", resolved, err)
	}
	corrected := appendRecord(t, store, &Answer{QuestionID: "decision-review", Target: target, Interpretation: AnswerDeferred, Supersedes: approved.ID})
	appendRecord(t, store, &StateChange{DecisionID: "latest", DecisionStatus: DecisionDeferred, AnswerID: corrected.ID})
	resolved, err = loadState(t, store).ResolveDecisions(DecisionContext{Valid: func(form.Decision) (bool, error) { return true, nil }})
	if err != nil || resolved[3].Status != DecisionDeferred || resolved[3].Valid {
		t.Fatalf("decision correction: %+v, %v", resolved, err)
	}
}

func TestFindingsCompletionAndExternalResults(t *testing.T) {
	store, _ := workStore(t)
	run := &StageRun{StageID: "implement", PlanVersion: 1, State: StageWorking}
	appendRecord(t, store, run)
	round := &Round{StageRunID: run.ID, SessionID: "session"}
	appendRecord(t, store, round)
	target := form.TargetVersion{Kind: form.TargetTree, Tree: "result"}
	norms := []form.NormReference{{Path: "rules", BlobID: "rules-a"}}
	appendRecord(t, store, &Judgment{RoundID: round.ID, Judgment: check.Judgment{JudgmentResult: form.JudgmentResult{
		Aspect: "test", TargetVersion: target, Norms: norms, Judgment: form.Fail,
		Findings: []form.Finding{{ID: "missing-test", Blocking: true, Kind: form.AutoFixable}},
	}, Runner: "judge"}})
	if findings := loadState(t, store).UnresolvedBlocking(run.ID); len(findings) != 1 {
		t.Fatalf("unresolved findings: %+v", findings)
	}
	appendRecord(t, store, &Judgment{RoundID: round.ID, Judgment: check.Judgment{JudgmentResult: form.JudgmentResult{
		Aspect: "test", TargetVersion: target, Norms: norms, Judgment: form.Pass,
		Findings: []form.Finding{{ID: "resolution", ResolvedFindingIDs: []string{"missing-test"}}},
	}, Runner: "judge"}})
	state := loadState(t, store)
	if findings := state.UnresolvedBlocking(run.ID); len(findings) != 0 {
		t.Fatalf("resolved findings retained: %+v", findings)
	}
	if judgments := state.LatestJudgments(target, norms); len(judgments) != 1 || judgments[0].Judgment != form.Pass {
		t.Fatalf("latest judgment: %+v", judgments)
	}
	changed := []form.NormReference{{Path: "rules", BlobID: "rules-b"}}
	appendRecord(t, store, &Judgment{RoundID: round.ID, Judgment: check.Judgment{JudgmentResult: form.JudgmentResult{
		Aspect: "test", TargetVersion: target, Norms: changed, Judgment: form.Fail,
	}, Runner: "judge"}})
	if judgments := loadState(t, store).LatestJudgments(target, norms); len(judgments) != 0 {
		t.Fatal("older judgment hid newer norm mismatch")
	}
	externalTarget := form.ExternalOperationTarget{OperationID: "publish", Kind: "comment", Target: "issue:42", InputHash: strings.Repeat("b", 64), DefinitionHash: strings.Repeat("c", 64), ExecutableHash: strings.Repeat("d", 64)}
	approvalTarget := form.ApprovalTarget{Kind: form.ApprovalExternal, External: &externalTarget}
	appendRecord(t, store, &Question{Question: form.Question{ID: "publish-question", ApprovalTarget: approvalTarget}})
	approval := appendRecord(t, store, &Approval{Answer: Answer{QuestionID: "publish-question", Target: approvalTarget, Interpretation: AnswerApproved}})
	before := appendRecord(t, store, &ExternalOp{ExternalOperationTarget: externalTarget, ApprovalID: approval.ID, Input: json.RawMessage(`{"body":"Result"}`), Status: ExternalRunning})
	appendRecord(t, store, &ExternalOp{ExternalOperationTarget: externalTarget, ApprovalID: approval.ID, BeforeID: before.ID, Input: json.RawMessage(`{"body":"Result"}`), Status: ExternalUnknown})
	state = loadState(t, store)
	if operations := state.CurrentPosition().ExternalOps; len(operations) != 1 || operations[0].Status != ExternalUnknown || operations[0].BeforeID != before.ID {
		t.Fatalf("external execution state: %+v", operations)
	}
	completed := *run
	completed.State, completed.Commit = StageCompleted, "commit"
	appendRecord(t, store, &completed)
	appendRecord(t, store, &StateChange{WorkStatus: WorkCompleted})
	state = loadState(t, store)
	if _, open := state.OpenRound(); open || len(state.IncompleteStageRuns()) != 0 || state.WorkStatus() != WorkCompleted {
		t.Fatal("completed work remained active")
	}
}

func TestExternalPlanApprovalIdentity(t *testing.T) {
	store, _ := workStore(t)
	target := form.ExternalOperationTarget{OperationID: "operation", Kind: "comment", Target: "issue:42", InputHash: strings.Repeat("b", 64), DefinitionHash: strings.Repeat("c", 64), ExecutableHash: strings.Repeat("d", 64)}
	executable := plan.Plan{Version: 1, ExternalOperations: []plan.ExternalOperation{{Kind: target.Kind, Target: target.Target, InputHash: target.InputHash, DefinitionHash: target.DefinitionHash, ExecutableHash: target.ExecutableHash}}}
	appendRecord(t, store, &PlanVersion{Plan: executable, Origin: plan.InitialProposal})
	approvalTarget := form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: 1}
	appendRecord(t, store, &Question{Question: form.Question{ID: "plan", ApprovalTarget: approvalTarget}})
	approval := appendRecord(t, store, &Approval{Answer: Answer{QuestionID: "plan", Target: approvalTarget, Interpretation: AnswerApproved}})
	state := loadState(t, store)
	if answer, ok := state.ResolveExternalApproval(target, &executable); !ok || answer.RecordID != approval.ID {
		t.Fatalf("plan declaration approval: %+v, %v", answer, ok)
	}
	changed := target
	changed.ExecutableHash = "changed"
	if _, ok := state.ResolveExternalApproval(changed, &executable); ok {
		t.Fatal("changed executable retained plan approval")
	}
	changed = target
	changed.InputHash = "changed"
	if _, ok := state.ResolveExternalApproval(changed, &executable); ok {
		t.Fatal("changed input retained plan approval")
	}
}
