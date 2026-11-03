package external

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// External executes registered commands in one checkout and records their outcomes.
// The caller holds the work lock throughout Execute and Recover. Engine owns
// session, branch, and executable-plan selection and creates approval questions.
type External struct {
	root          string
	store         *fsstore.Store
	validator     *ledger.Validator
	registrations map[string]config.ExternalOperation
	run           func(context.Context, config.Command, json.RawMessage, string) commandResult
}

// Request supplies the concrete target displayed for approval and the JSON input.
// Plan is the executable plan selected by engine, or nil for individual approval.
type Request struct {
	Kind          string
	Target        string
	Input         json.RawMessage
	InstructionID string
	Plan          *plan.Plan
}

// Result returns the latest operation record, including its ledger record ID.
// ConfirmationRequired asks engine to obtain human guidance without reexecution.
type Result struct {
	RecordID             string
	Operation            ledger.ExternalOp
	ConfirmationRequired bool
}

// ApprovalRequired reports the current target for a new question owned by engine.
type ApprovalRequired struct {
	Target form.ExternalOperationTarget
}

func (e *ApprovalRequired) Error() string {
	return fmt.Sprintf("external operation %s on %s requires approval", e.Target.Kind, e.Target.Target)
}

// New receives registrations already loaded and validated by config.
// root is the absolute checkout root; store belongs to the current work.
func New(root string, store *fsstore.Store, validator *ledger.Validator, registrations map[string]config.ExternalOperation) *External {
	e := &External{root: root, store: store, validator: validator, registrations: registrations}
	e.run = e.runCommand
	return e
}

// Describe identifies an operation for a question or plan declaration.
// Input hashes cover the supplied JSON bytes. OccurrenceField names a top-level
// input field; its non-null value is also included in the operation ID.
func (e *External) Describe(request Request) (form.ExternalOperationTarget, error) {
	registration, ok := e.registrations[request.Kind]
	if !ok {
		return form.ExternalOperationTarget{}, fmt.Errorf("external operation %q is not registered", request.Kind)
	}
	identity, err := operationIdentity(request, registration)
	if err != nil {
		return form.ExternalOperationTarget{}, err
	}
	definition, err := json.Marshal(registration)
	if err != nil {
		return form.ExternalOperationTarget{}, err
	}
	path, err := e.executablePath(registration.Executable)
	if err != nil {
		return form.ExternalOperationTarget{}, err
	}
	executable, err := os.ReadFile(path)
	if err != nil {
		return form.ExternalOperationTarget{}, fmt.Errorf("read external executable: %w", err)
	}
	return form.ExternalOperationTarget{
		OperationID: identity, Kind: request.Kind, Target: request.Target,
		InputHash: contentHash(request.Input), DefinitionHash: contentHash(definition),
		ExecutableHash: contentHash(executable),
	}, nil
}

// Execute rechecks approval immediately before recording and starting an operation.
// Repeated IDs return the recorded result, including failed, running, and unknown
// outcomes. Recovery is explicit and never invokes the write command again.
func (e *External) Execute(ctx context.Context, request Request) (Result, error) {
	registration, ok := e.registrations[request.Kind]
	if !ok {
		return Result{}, fmt.Errorf("external operation %q is not registered", request.Kind)
	}
	id, err := operationIdentity(request, registration)
	if err != nil {
		return Result{}, err
	}
	state, err := ledger.Load(e.store, nil, e.validator)
	if err != nil {
		return Result{}, err
	}
	if _, ok := state.ExternalOperation(id); ok {
		return e.recorded(id)
	}
	if _, _, paused := state.Paused(); paused {
		return Result{}, errors.New("work is paused; use devkit resume before starting an external operation")
	}
	if state.WorkStatus() != ledger.WorkActive {
		return Result{}, errors.New("work has ended")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	target, err := e.Describe(request)
	if err != nil {
		return Result{}, err
	}
	approval, ok := state.ResolveExternalApproval(target, request.Plan)
	if !ok {
		return Result{}, &ApprovalRequired{Target: target}
	}
	before := ledger.ExternalOp{
		ExternalOperationTarget: target, ApprovalID: approval.RecordID,
		Input: append(json.RawMessage(nil), request.Input...), Status: ledger.ExternalRunning,
		InstructionID: request.InstructionID,
	}
	result, err := e.append(before)
	if err != nil {
		return Result{}, err
	}
	outcome := e.invoke(ctx, registration.Command, before.Input, id)
	return e.finish(result, outcome)
}

// Recover queries an interrupted or unknown operation when a query is registered.
// The caller holds the work lock, so a running record here has no active executor.
// Recovery may record a result during pause and never clears the pause.
func (e *External) Recover(ctx context.Context, operationID string) (Result, error) {
	result, err := e.recorded(operationID)
	if err != nil {
		return Result{}, err
	}
	op := result.Operation
	if op.Status != ledger.ExternalRunning && op.Status != ledger.ExternalUnknown {
		return result, nil
	}
	registration, ok := e.registrations[op.Kind]
	if !ok || registration.Query == nil {
		return e.unresolved(result, "No registered query can resolve this operation; human confirmation is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	outcome := e.invoke(ctx, *registration.Query, op.Input, operationID)
	if outcome.notStarted {
		outcome.Status = ledger.ExternalUnknown
	}
	return e.finish(result, outcome)
}

func (e *External) unresolved(result Result, message string) (Result, error) {
	if result.Operation.Status == ledger.ExternalUnknown {
		result.ConfirmationRequired = true
		return result, nil
	}
	return e.finish(result, commandResult{Status: ledger.ExternalUnknown, Result: message})
}

func (e *External) recorded(id string) (Result, error) {
	records, err := ledger.List(e.store, e.validator)
	if err != nil {
		return Result{}, err
	}
	for i := len(records) - 1; i >= 0; i-- {
		if op, ok := records[i].Payload.(*ledger.ExternalOp); ok && op.OperationID == id {
			return Result{RecordID: records[i].ID, Operation: *op, ConfirmationRequired: op.Status == ledger.ExternalUnknown}, nil
		}
	}
	return Result{}, fmt.Errorf("external operation %q is not recorded", id)
}

func (e *External) append(op ledger.ExternalOp) (Result, error) {
	record, err := ledger.NewRecord(e.store, &op)
	if err != nil {
		return Result{}, err
	}
	if err := ledger.Append(e.store, record, e.validator); err != nil {
		return Result{}, err
	}
	return Result{RecordID: record.ID, Operation: op, ConfirmationRequired: op.Status == ledger.ExternalUnknown}, nil
}

func (e *External) finish(before Result, outcome commandResult) (Result, error) {
	op := before.Operation
	if op.BeforeID == "" {
		op.BeforeID = before.RecordID
	}
	op.Status, op.Result, op.ExternalID = outcome.Status, outcome.Result, outcome.ExternalID
	result, err := e.append(op)
	if err != nil {
		return before, fmt.Errorf("record external operation outcome: %w", err)
	}
	return result, nil
}

func operationIdentity(request Request, registration config.ExternalOperation) (string, error) {
	if request.Kind == "" || request.Target == "" || !json.Valid(request.Input) {
		return "", errors.New("external operation kind, target, and valid JSON input are required")
	}
	var occurrence json.RawMessage
	if registration.OccurrenceField != "" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(request.Input, &fields); err != nil {
			return "", fmt.Errorf("read occurrence field: %w", err)
		}
		occurrence = fields[registration.OccurrenceField]
		if len(occurrence) == 0 || string(occurrence) == "null" {
			return "", fmt.Errorf("external operation input requires occurrence field %q", registration.OccurrenceField)
		}
	}
	data, err := json.Marshal(struct {
		Kind       string          `json:"kind"`
		Target     string          `json:"target"`
		InputHash  string          `json:"input_hash"`
		Occurrence json.RawMessage `json:"occurrence,omitempty"`
	}{request.Kind, request.Target, contentHash(request.Input), occurrence})
	if err != nil {
		return "", err
	}
	return contentHash(data), nil
}

func (e *External) executablePath(name string) (string, error) {
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.root, path)
	}
	rel, err := filepath.Rel(e.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("external executable %q must be inside the checkout", name)
	}
	return path, nil
}

func contentHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
