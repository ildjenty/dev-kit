package external

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func fixture(t *testing.T) (*External, Request) {
	t.Helper()
	sources := make(map[string][]byte)
	for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "kit", "schemas", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = data
	}
	v, err := ledger.NewValidator(sources)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "write"), []byte("command version one"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := New(root, fsstore.New(filepath.Join(root, "run")), v, map[string]config.ExternalOperation{
		"publish": {Command: config.Command{Executable: "write", Args: []string{"fixed"}, TimeoutSeconds: 1}},
	})
	lock, err := e.store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lock.Close(); err != nil {
			t.Error(err)
		}
	})
	return e, Request{Kind: "publish", Target: "repository/item", Input: json.RawMessage(`{"message":"Hello","occurrence":"first"}`)}
}

func appendRecord(t *testing.T, e *External, payload ledger.Payload) ledger.Record {
	t.Helper()
	r, err := ledger.NewRecord(e.store, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Append(e.store, r, e.validator); err != nil {
		t.Fatal(err)
	}
	return r
}

func approve(t *testing.T, e *External, request Request) string {
	t.Helper()
	target, err := e.Describe(request)
	if err != nil {
		t.Fatal(err)
	}
	approvalTarget := form.ApprovalTarget{Kind: form.ApprovalExternal, External: &target}
	appendRecord(t, e, &ledger.Question{Question: form.Question{
		ID: "approval-" + target.OperationID + target.DefinitionHash + target.ExecutableHash, Text: "Approve publication?",
		ApprovalTarget: approvalTarget, Options: []string{"approve", "reject", "defer"},
		StopCategory: "approval", StopReason: "Publication requires approval", Evidence: []form.Reference{},
	}})
	r := appendRecord(t, e, &ledger.Approval{Answer: ledger.Answer{
		QuestionID: "approval-" + target.OperationID + target.DefinitionHash + target.ExecutableHash,
		Target:     approvalTarget, Interpretation: ledger.AnswerApproved, Original: "Approved",
		SessionID: "session", ObservedAt: time.Now(),
	}})
	return r.ID
}

func TestExecuteRecordsAndDeduplicates(t *testing.T) {
	e, request := fixture(t)
	approvalID := approve(t, e, request)
	calls := 0
	e.run = func(ctx context.Context, command config.Command, input json.RawMessage, id string) commandResult {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("command has no deadline")
		}
		s, err := ledger.Load(e.store, nil, e.validator)
		if err != nil {
			t.Fatal(err)
		}
		before, ok := s.ExternalOperation(id)
		if !ok || before.Status != ledger.ExternalRunning || before.ApprovalID != approvalID || string(input) != string(request.Input) {
			t.Fatalf("missing before record or input: %+v", before)
		}
		return commandResult{Status: ledger.ExternalSucceeded, Result: "Published", ExternalID: "item-123"}
	}
	result, err := e.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation.Status != ledger.ExternalSucceeded || result.Operation.BeforeID == "" || result.Operation.ExternalID != "item-123" {
		t.Fatalf("unexpected result: %+v", result)
	}
	// Recorded success remains usable even if the executable has disappeared.
	if err := os.Remove(filepath.Join(e.root, "write")); err != nil {
		t.Fatal(err)
	}
	again, err := e.Execute(context.Background(), request)
	if err != nil || again.RecordID != result.RecordID || calls != 1 {
		t.Fatalf("operation was not deduplicated: %+v, %v, calls=%d", again, err, calls)
	}
}

func TestApprovalMismatch(t *testing.T) {
	for _, change := range []string{"none", "target", "input", "definition", "executable"} {
		t.Run(change, func(t *testing.T) {
			e, request := fixture(t)
			if change != "none" {
				approve(t, e, request)
			}
			switch change {
			case "target":
				request.Target = "repository/other"
			case "input":
				request.Input = json.RawMessage(`{"message":"Changed"}`)
			case "definition":
				r := e.registrations[request.Kind]
				r.Args = []string{"changed"}
				e.registrations[request.Kind] = r
			case "executable":
				if err := os.WriteFile(filepath.Join(e.root, "write"), []byte("command version two"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
				t.Fatal("unapproved command was executed")
				return commandResult{}
			}
			_, err := e.Execute(context.Background(), request)
			var required *ApprovalRequired
			if !errors.As(err, &required) || required.Target.Target != request.Target {
				t.Fatalf("expected current approval target, got %v", err)
			}
		})
	}
}

func TestUnknownRecoveryAndPause(t *testing.T) {
	e, request := fixture(t)
	r := e.registrations[request.Kind]
	r.Query = &config.Command{Executable: "write", Args: []string{"query"}, TimeoutSeconds: 2}
	e.registrations[request.Kind] = r
	approve(t, e, request)
	e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
		return commandResult{Status: ledger.ExternalUnknown, Result: "Response was lost"}
	}
	unknown, err := e.Execute(context.Background(), request)
	if err != nil || !unknown.ConfirmationRequired {
		t.Fatalf("expected unknown outcome: %+v, %v", unknown, err)
	}
	pause := appendRecord(t, e, &ledger.Pause{Position: form.Reference{RecordID: unknown.RecordID}})
	calls := 0
	e.run = func(ctx context.Context, command config.Command, input json.RawMessage, id string) commandResult {
		calls++
		if command.Args[0] != "query" || id != unknown.Operation.OperationID || string(input) != string(request.Input) {
			t.Fatalf("incorrect query: %+v, %s, %s", command, input, id)
		}
		return commandResult{Status: ledger.ExternalSucceeded, ExternalID: "item-123"}
	}
	again, err := e.Execute(context.Background(), request)
	if err != nil || again.RecordID != unknown.RecordID || calls != 0 {
		t.Fatalf("unknown operation was repeated: %+v, %v", again, err)
	}
	recovered, err := e.Recover(context.Background(), unknown.Operation.OperationID)
	if err != nil || recovered.Operation.Status != ledger.ExternalSucceeded || recovered.Operation.BeforeID != unknown.Operation.BeforeID || calls != 1 {
		t.Fatalf("incorrect recovery: %+v, %v", recovered, err)
	}
	s, err := ledger.Load(e.store, nil, e.validator)
	if err != nil {
		t.Fatal(err)
	}
	if _, id, paused := s.Paused(); !paused || id != pause.ID {
		t.Fatal("recovery cleared the pause")
	}
	request.Input = json.RawMessage(`{"message":"Another operation"}`)
	approve(t, e, request)
	e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
		calls++
		return commandResult{Status: ledger.ExternalSucceeded}
	}
	if _, err := e.Execute(context.Background(), request); err == nil || err.Error() != "work is paused; use devkit resume before starting an external operation" {
		t.Fatalf("expected pause rejection: %v", err)
	}
	if calls != 1 {
		t.Fatalf("new operation started during pause: calls=%d", calls)
	}
}

func TestExecuteRejectsPausedWork(t *testing.T) {
	e, request := fixture(t)
	approvalID := approve(t, e, request)
	appendRecord(t, e, &ledger.Pause{Position: form.Reference{RecordID: approvalID}})
	calls := 0
	e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
		calls++
		return commandResult{Status: ledger.ExternalSucceeded}
	}
	_, err := e.Execute(context.Background(), request)
	if err == nil || err.Error() != "work is paused; use devkit resume before starting an external operation" {
		t.Fatalf("expected pause rejection before invocation: %v", err)
	}
	if calls != 0 {
		t.Fatalf("operation started after pause publication: calls=%d", calls)
	}
}

func TestRecoveryRequiresOriginalDefinition(t *testing.T) {
	for _, status := range []ledger.ExternalStatus{ledger.ExternalRunning, ledger.ExternalUnknown} {
		t.Run(string(status), func(t *testing.T) {
			e, request := fixture(t)
			r := e.registrations[request.Kind]
			r.Query = &config.Command{Executable: "write", Args: []string{"query"}, TimeoutSeconds: 1}
			e.registrations[request.Kind] = r
			approvalID := approve(t, e, request)
			target, err := e.Describe(request)
			if err != nil {
				t.Fatal(err)
			}
			before := appendRecord(t, e, &ledger.ExternalOp{
				ExternalOperationTarget: target, ApprovalID: approvalID, Input: request.Input, Status: ledger.ExternalRunning,
			})
			if status == ledger.ExternalUnknown {
				appendRecord(t, e, &ledger.ExternalOp{
					ExternalOperationTarget: target, ApprovalID: approvalID, BeforeID: before.ID,
					Input: request.Input, Status: status,
				})
			}
			r.Query.Args = []string{"different-query"}
			e.registrations[request.Kind] = r
			e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
				t.Fatal("changed query was invoked")
				return commandResult{}
			}
			result, err := e.Recover(context.Background(), target.OperationID)
			if err != nil || result.Operation.Status != ledger.ExternalUnknown || !result.ConfirmationRequired || result.Operation.BeforeID != before.ID {
				t.Fatalf("changed definition resolved the operation: %+v, %v", result, err)
			}
		})
	}
}

func TestInterruptedWithoutQuery(t *testing.T) {
	e, request := fixture(t)
	approvalID := approve(t, e, request)
	target, err := e.Describe(request)
	if err != nil {
		t.Fatal(err)
	}
	before := appendRecord(t, e, &ledger.ExternalOp{
		ExternalOperationTarget: target, ApprovalID: approvalID, Input: request.Input, Status: ledger.ExternalRunning,
	})
	e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
		t.Fatal("interrupted operation was executed again")
		return commandResult{}
	}
	running, err := e.Execute(context.Background(), request)
	if err != nil || running.RecordID != before.ID || running.Operation.Status != ledger.ExternalRunning {
		t.Fatalf("expected recorded running state: %+v, %v", running, err)
	}
	result, err := e.Recover(context.Background(), target.OperationID)
	if err != nil || !result.ConfirmationRequired || result.Operation.Status != ledger.ExternalUnknown || result.Operation.BeforeID != before.ID {
		t.Fatalf("expected human confirmation: %+v, %v", result, err)
	}
}

func TestOccurrenceIdentity(t *testing.T) {
	e, request := fixture(t)
	r := e.registrations[request.Kind]
	r.OccurrenceField = "occurrence"
	e.registrations[request.Kind] = r
	first, err := e.Describe(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Input = json.RawMessage(`{"message":"Hello","occurrence":"second"}`)
	second, err := e.Describe(request)
	if err != nil || first.OperationID == second.OperationID {
		t.Fatalf("distinct occurrences must have distinct IDs: %v", err)
	}
	request.Input = json.RawMessage(`{"message":"Hello"}`)
	if _, err := e.Describe(request); err == nil {
		t.Fatal("missing occurrence was accepted")
	}
}

func TestCommandBoundary(t *testing.T) {
	e, request := fixture(t)
	script := "#!/bin/sh\ncat > received.json\nprintf '%s' \"$DEVKIT_OPERATION_ID\" > operation-id\nprintf '%s' \"$1\" > argument\nprintf '%s' '{\"status\":\"succeeded\",\"external_id\":\"remote-1\"}'\n"
	if err := os.WriteFile(filepath.Join(e.root, "write"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := e.registrations[request.Kind]
	r.Args = []string{"$(must-not-expand) *"}
	// Allow extra time so a loaded machine does not turn this boundary test into a timeout test.
	r.TimeoutSeconds = 10
	e.registrations[request.Kind] = r
	approve(t, e, request)
	result, err := e.Execute(context.Background(), request)
	if err != nil || result.Operation.Status != ledger.ExternalSucceeded {
		t.Fatalf("command failed: %+v, %v", result, err)
	}
	for name, want := range map[string]string{"received.json": string(request.Input), "operation-id": result.Operation.OperationID, "argument": r.Args[0]} {
		data, err := os.ReadFile(filepath.Join(e.root, name))
		if err != nil || string(data) != want {
			t.Fatalf("%s: got %q, want %q, error=%v", name, data, want, err)
		}
	}
}

func TestExecutableSymlinks(t *testing.T) {
	for _, location := range []string{"inside", "outside"} {
		t.Run(location, func(t *testing.T) {
			e, request := fixture(t)
			outside := location == "outside"
			directory := e.root
			if outside {
				directory = t.TempDir()
			}
			target := filepath.Join(directory, "command")
			if err := os.WriteFile(target, []byte("#!/bin/sh\nprintf '%s' '{\"status\":\"succeeded\"}'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(e.root, "linked-command")); err != nil {
				t.Fatal(err)
			}
			r := e.registrations[request.Kind]
			r.Executable = "linked-command"
			e.registrations[request.Kind] = r
			if _, err := e.Describe(request); (err != nil) != outside {
				t.Fatalf("unexpected approval path result: %v", err)
			}
			result := e.invoke(context.Background(), r.Command, request.Input, "operation")
			if outside {
				if result.Status != ledger.ExternalFailed || !result.notStarted {
					t.Fatalf("outside executable was not rejected: %+v", result)
				}
			} else if result.Status != ledger.ExternalSucceeded {
				t.Fatalf("inside executable failed: %+v", result)
			}
		})
	}
}

func TestInvokeDeadlineOverridesReportedSuccess(t *testing.T) {
	e, request := fixture(t)
	e.run = func(ctx context.Context, _ config.Command, _ json.RawMessage, _ string) commandResult {
		<-ctx.Done()
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expected deadline expiration, got %v", ctx.Err())
		}
		return commandResult{Status: ledger.ExternalSucceeded}
	}
	result := e.invoke(context.Background(), e.registrations[request.Kind].Command, request.Input, "operation")
	if result.Status != ledger.ExternalUnknown || !strings.Contains(result.Result, context.DeadlineExceeded.Error()) {
		t.Fatalf("deadline expiration did not override success: %+v", result)
	}
}

func TestCommandOutcomes(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		status ledger.ExternalStatus
	}{
		{"failed", "printf '%s' '{\"status\":\"failed\",\"result\":\"Rejected\"}'", ledger.ExternalFailed},
		{"invalid", "printf '%s' 'not JSON'", ledger.ExternalUnknown},
		{"nonzero", "exit 1", ledger.ExternalUnknown},
		{"timeout", ": > started\nexec sleep 5", ledger.ExternalUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			e, request := fixture(t)
			if test.name == "timeout" {
				sleep, err := exec.LookPath("sleep")
				if err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", filepath.Dir(sleep))
			}
			if err := os.WriteFile(filepath.Join(e.root, "write"), []byte("#!/bin/sh\n"+test.script+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			approve(t, e, request)
			result, err := e.Execute(context.Background(), request)
			if err != nil || result.Operation.Status != test.status {
				t.Fatalf("unexpected outcome: %+v, %v", result, err)
			}
			if test.name == "timeout" {
				if _, err := os.Stat(filepath.Join(e.root, "started")); err != nil {
					t.Fatalf("command did not start: %v", err)
				}
				if !strings.Contains(result.Operation.Result, context.DeadlineExceeded.Error()) {
					t.Fatalf("expected deadline expiration, got %q", result.Operation.Result)
				}
			}
			again, err := e.Execute(context.Background(), request)
			if err != nil || again.RecordID != result.RecordID {
				t.Fatalf("recorded outcome was not reused: %+v, %v", again, err)
			}
		})
	}
}

func TestQueryStartFailureDoesNotDetermineOriginalOutcome(t *testing.T) {
	e, request := fixture(t)
	r := e.registrations[request.Kind]
	r.Query = &config.Command{Executable: "missing-query", TimeoutSeconds: 1}
	e.registrations[request.Kind] = r
	approvalID := approve(t, e, request)
	target, err := e.Describe(request)
	if err != nil {
		t.Fatal(err)
	}
	appendRecord(t, e, &ledger.ExternalOp{
		ExternalOperationTarget: target, ApprovalID: approvalID, Input: request.Input, Status: ledger.ExternalRunning,
	})
	result, err := e.Recover(context.Background(), target.OperationID)
	if err != nil || result.Operation.Status != ledger.ExternalUnknown || !result.ConfirmationRequired {
		t.Fatalf("query startup failure determined the original outcome: %+v, %v", result, err)
	}
}

func TestPlanApprovalAndCorrection(t *testing.T) {
	e, request := fixture(t)
	target, err := e.Describe(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Plan = &plan.Plan{Version: 1, ExternalOperations: []plan.ExternalOperation{{
		Kind: target.Kind, Target: target.Target, InputHash: target.InputHash,
		DefinitionHash: target.DefinitionHash, ExecutableHash: target.ExecutableHash,
	}}}
	approvalTarget := form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: 1}
	appendRecord(t, e, &ledger.Question{Question: form.Question{
		ID: "plan-approval", Text: "Approve the plan?", ApprovalTarget: approvalTarget,
		Options: []string{"approve", "reject"}, StopCategory: "approval",
		StopReason: "External operations require plan approval", Evidence: []form.Reference{},
	}})
	approval := appendRecord(t, e, &ledger.Approval{Answer: ledger.Answer{
		QuestionID: "plan-approval", Target: approvalTarget, Interpretation: ledger.AnswerApproved,
		Original: "Approved", SessionID: "session", ObservedAt: time.Now(),
	}})
	correction := ledger.Answer{
		QuestionID: "plan-approval", Target: approvalTarget, Interpretation: ledger.AnswerRejected,
		Original: "Rejected", SessionID: "session", ObservedAt: time.Now(), Supersedes: approval.ID,
	}
	rejected := appendRecord(t, e, &correction)
	_, err = e.Execute(context.Background(), request)
	var required *ApprovalRequired
	if !errors.As(err, &required) {
		t.Fatalf("corrected approval was accepted: %v", err)
	}
	correction.Interpretation, correction.Original, correction.Supersedes = ledger.AnswerApproved, "Approved again", rejected.ID
	reapproved := appendRecord(t, e, &ledger.Approval{Answer: correction})
	e.run = func(context.Context, config.Command, json.RawMessage, string) commandResult {
		return commandResult{Status: ledger.ExternalSucceeded}
	}
	result, err := e.Execute(context.Background(), request)
	if err != nil || result.Operation.ApprovalID != reapproved.ID {
		t.Fatalf("plan approval was not used: %+v, %v", result, err)
	}
}
