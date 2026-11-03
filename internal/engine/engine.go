package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/external"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/workspace"
	"github.com/oklog/ulid/v2"
)

// Engine advances one checkout. SessionID is the current host session, not a work ID.
// Each operation reloads records; subagents are always launched by the caller.
type Engine struct {
	workspace *workspace.Workspace
	sessionID string
	version   string
	schemas   map[string][]byte
	forms     map[string]*form.Schema
	plans     *plan.Validator
	validator *ledger.Validator
	workID    string
}

// StartRequest identifies the repository template and the task supplied by the human.
type StartRequest struct {
	Template string
	Task     string
}

// Result is a short response. Instruction bodies are written to Brief files.
// A paused response deliberately has no instruction.
type Result struct {
	WorkID      string            `json:"work_id"`
	Status      ledger.WorkStatus `json:"status"`
	Paused      bool              `json:"paused"`
	Message     string            `json:"message,omitempty"`
	Warning     string            `json:"warning,omitempty"`
	Instruction *form.Instruction `json:"instruction,omitempty"`
	External    *external.Result  `json:"external,omitempty"`
}

// Status includes the full ledger-derived current position and pending instructions.
type Status struct {
	WorkID       string               `json:"work_id"`
	Position     ledger.Position      `json:"position"`
	Progress     *ledger.Progress     `json:"progress,omitempty"`
	Questions    []form.Question      `json:"questions"`
	Instructions []ledger.Instruction `json:"instructions"`
	Warning      string               `json:"warning,omitempty"`
}

// New opens a checkout. Forward operations load schemas from its kit copy. An empty session ID
// uses CLAUDE_CODE_SESSION_ID, then CODEX_THREAD_ID. Pause also works without a session.
func New(ctx context.Context, cwd, sessionID string) (*Engine, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		sessionID = os.Getenv("CLAUDE_CODE_SESSION_ID")
		if sessionID == "" {
			sessionID = os.Getenv("CODEX_THREAD_ID")
		}
	}
	if sessionID != "" && !localName(sessionID) {
		return nil, fmt.Errorf("invalid session ID: %q", sessionID)
	}
	return &Engine{workspace: w, sessionID: sessionID, version: kit.Version}, nil
}

func (e *Engine) initialize() error {
	if e.plans != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		return err
	}
	settings, err := config.Parse(data)
	if err != nil {
		return err
	}
	k, err := kit.Open(e.Root(), settings.KitVersion)
	if err != nil {
		return err
	}
	e.schemas = map[string][]byte{}
	for _, name := range schemaNames {
		e.schemas[name], err = k.ReadFile("schemas/" + name + ".json")
		if err != nil {
			return err
		}
	}
	validator, err := ledger.NewValidator(e.schemas)
	if err != nil {
		return err
	}
	plans, err := plan.NewValidator(e.schemas["plan-template"], e.schemas["plan"])
	if err != nil {
		return err
	}
	resources := map[string][]byte{}
	for name, data := range e.schemas {
		resources["urn:devkit:form:"+name] = data
	}
	forms := map[string]*form.Schema{}
	for _, name := range []string{"progress", "planner-result", "judgment", "detection-result"} {
		forms[name], err = form.CompileSchemaWithResources(e.schemas[name], resources)
		if err != nil {
			return err
		}
	}
	e.validator, e.plans, e.forms = validator, plans, forms
	return nil
}

var schemaNames = []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan", "planner-result", "detection-result"}

// Historical record validation remains available while forward assets are edited.
// Forward operations replace this validator with the working kit schemas.
func (e *Engine) recordValidator() error {
	if e.validator != nil {
		return nil
	}
	records, err := e.store().List()
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Kind != string(ledger.KindWork) {
			continue
		}
		var work ledger.Work
		if err := fsstore.DecodePayload(record.Payload, &work); err != nil {
			return err
		}
		if work.KitVersion != e.version {
			return errors.New("work and kit versions do not match")
		}
		sources := map[string][]byte{}
		for _, name := range schemaNames {
			if name == "planner-result" || name == "detection-result" {
				continue
			}
			sources[name], err = e.asset(context.Background(), work.BaseCommit, "schemas/"+name+".json")
			if err != nil {
				return err
			}
		}
		e.validator, err = ledger.NewValidator(sources)
		return err
	}
	return errors.New("work record is missing")
}

func localName(name string) bool {
	return name != "." && filepath.IsLocal(name) && !strings.ContainsAny(name, "/\\")
}

// Root returns the resolved checkout root.
func (e *Engine) Root() string          { return e.workspace.Root() }
func (e *Engine) runRoot() string       { return filepath.Join(e.Root(), ".devkit/run") }
func (e *Engine) workRoot() string      { return filepath.Join(e.runRoot(), "work", e.workID) }
func (e *Engine) store() *fsstore.Store { return fsstore.New(e.workRoot()) }
func hash(data []byte) string           { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func (e *Engine) append(payload ledger.Payload) (string, error) {
	r, err := ledger.NewRecord(e.store(), payload)
	if err != nil {
		return "", err
	}
	switch p := payload.(type) {
	case *ledger.StageRun:
		if p.ID == "" {
			p.ID = r.ID
		}
	case *ledger.Round:
		if p.ID == "" {
			p.ID = r.ID
		}
	}
	return r.ID, ledger.Append(e.store(), r, e.validator)
}
func (e *Engine) load() (*ledger.State, error) {
	if err := e.recordValidator(); err != nil {
		return nil, err
	}
	stops, err := hook.ReadStops(e.workRoot())
	if err != nil {
		return nil, err
	}
	return ledger.Load(e.store(), stops, e.validator)
}
func (e *Engine) resolve(ctx context.Context, bind bool) (*ledger.State, string, error) {
	var obs *hook.SessionObservation
	var err error
	if e.sessionID != "" {
		obs, err = hook.ReadSession(e.runRoot(), e.sessionID)
		if err != nil {
			return nil, "", err
		}
	}
	id := ""
	if obs != nil {
		id = obs.WorkID
	}
	branch, branchErr := e.workspace.Branch(ctx)
	if id == "" && strings.HasPrefix(branch, "devkit/") {
		id = strings.TrimPrefix(branch, "devkit/")
	}
	if id == "" || !localName(id) {
		return nil, "", errors.New("no work is bound to this session or branch")
	}
	e.workID = id
	state, err := e.load()
	if err != nil {
		return nil, "", err
	}
	work, ok := state.Work()
	if !ok || work.ID != id {
		return nil, "", errors.New("work record does not match the work directory")
	}
	if work.KitVersion != e.version {
		return nil, "", errors.New("work and kit versions do not match")
	}
	warning := ""
	if branchErr != nil || branch != work.Branch {
		warning = fmt.Sprintf("Current branch does not match work branch %s.", work.Branch)
	}
	if bind && warning == "" && (obs == nil || obs.WorkID == "") {
		if err := e.checkSession(); err != nil {
			return nil, "", err
		}
		obs, err = hook.ReadSession(e.runRoot(), e.sessionID)
		if err != nil {
			return nil, "", err
		}
		bound := *obs
		bound.WorkID = id
		bound.ObservedAt = time.Now().UTC()
		if err := hook.PublishSession(e.runRoot(), bound, "bind"); err != nil {
			return nil, "", err
		}
	}
	return state, warning, nil
}
func (e *Engine) checkSession() error {
	if e.sessionID == "" {
		return errors.New("session ID is missing; run devkit doctor to check hook activity")
	}
	entries, err := os.ReadDir(filepath.Join(e.runRoot(), "sessions", e.sessionID))
	if err != nil {
		return fmt.Errorf("SessionStart observation is missing; run devkit doctor: %w", err)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if !strings.HasSuffix(entries[i].Name(), "-SessionStart.json") {
			continue
		}
		var obs hook.SessionObservation
		data, err := os.ReadFile(filepath.Join(e.runRoot(), "sessions", e.sessionID, entries[i].Name()))
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &obs); err != nil {
			return err
		}
		if obs.SessionID != e.sessionID {
			return errors.New("SessionStart observation has a different session ID")
		}
		return nil
	}
	return errors.New("SessionStart observation is missing; run devkit doctor to check hook settings")
}
func response(state *ledger.State) Result {
	p := state.CurrentPosition()
	r := Result{Status: p.Status, Paused: p.Paused && p.Status == ledger.WorkActive}
	if p.Work != nil {
		r.WorkID = p.Work.ID
	}
	if r.Paused {
		r.Message = "Work is paused. Use devkit resume to resume."
	}
	return r
}
func (e *Engine) active() error {
	state, err := e.load()
	if err != nil {
		return err
	}
	if state.WorkStatus() != ledger.WorkActive {
		return errors.New("work has ended")
	}
	if _, _, paused := state.Paused(); paused {
		return errors.New("work is paused; use devkit resume")
	}
	return nil
}
func (e *Engine) mutation(ctx context.Context) (*fsstore.Lock, error) {
	_, warning, err := e.resolve(ctx, false)
	if err != nil {
		return nil, err
	}
	if warning != "" {
		return nil, errors.New(warning)
	}
	if err = e.checkSession(); err != nil {
		return nil, err
	}
	lock, err := e.store().Lock()
	if err != nil {
		return nil, err
	}
	if err = e.active(); err != nil {
		lock.Close()
		return nil, err
	}
	if err = e.initialize(); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}

// Start creates a work branch and a planner instruction from a repository template.
func (e *Engine) Start(ctx context.Context, request StartRequest) (Result, error) {
	if err := e.initialize(); err != nil {
		return Result{}, err
	}
	if err := e.checkSession(); err != nil {
		return Result{}, err
	}
	obs, err := hook.ReadSession(e.runRoot(), e.sessionID)
	if err != nil {
		return Result{}, err
	}
	if obs.WorkID != "" {
		e.workID = obs.WorkID
		s, err := e.load()
		if err != nil {
			return Result{}, err
		}
		if s.WorkStatus() == ledger.WorkActive {
			return Result{}, errors.New("the session is already bound to active work")
		}
	}
	name := request.Template
	if name == "" {
		name = "default"
	}
	if !localName(name) || strings.TrimSpace(request.Task) == "" {
		return Result{}, errors.New("a template name and task are required")
	}
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return Result{}, err
	}
	source, err := e.workspace.ReadFile(ctx, head, ".devkit/plans/"+name+".json")
	if err != nil {
		return Result{}, err
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		return Result{}, err
	}
	if template.Name != name {
		return Result{}, errors.New("template name does not match its file name")
	}
	e.workID = ulid.Make().String()
	lock, err := e.store().Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	branch := "devkit/" + e.workID
	if err = e.workspace.CreateBranch(ctx, branch); err != nil {
		return Result{}, err
	}
	if err = e.store().Publish("inbox/task.txt", []byte(request.Task)); err != nil {
		return Result{}, err
	}
	_, err = e.append(&ledger.Work{ID: e.workID, TaskRef: "inbox/task.txt", Branch: branch, BaseCommit: head, KitVersion: e.version})
	if err != nil {
		return Result{}, err
	}
	bound := *obs
	bound.WorkID = e.workID
	bound.ObservedAt = time.Now().UTC()
	if err = hook.PublishSession(e.runRoot(), bound, "bind"); err != nil {
		return Result{}, err
	}
	return e.planner(ctx, template, source, 1, plan.InitialProposal)
}

// Status reads current state without writing records or binding a new session.
func (e *Engine) Status(ctx context.Context) (Status, error) {
	state, warning, err := e.resolve(ctx, false)
	if err != nil {
		return Status{}, err
	}
	progress, _ := state.LatestProgress()
	return Status{WorkID: e.workID, Position: state.CurrentPosition(), Progress: progress, Questions: state.UnansweredQuestions(), Instructions: state.UnacceptedInstructions(), Warning: warning}, nil
}

// Pause publishes the independent pause overlay without acquiring the work lock.
func (e *Engine) Pause(ctx context.Context, reason string) (Result, error) {
	state, warning, err := e.resolve(ctx, false)
	if err != nil {
		return Result{}, err
	}
	if state.WorkStatus() == ledger.WorkActive {
		if _, _, paused := state.Paused(); !paused {
			_, err = e.append(&ledger.Pause{Position: form.Reference{RecordID: state.LastRecordID()}, Reason: reason})
			if err != nil {
				return Result{}, err
			}
		}
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	r := response(state)
	r.Warning = warning
	return r, nil
}

// Resume records an explicit human request to resume. It never starts new work itself.
func (e *Engine) Resume(ctx context.Context) (Result, error) {
	_, warning, err := e.resolve(ctx, true)
	if err != nil {
		return Result{}, err
	}
	if warning != "" {
		return Result{}, errors.New(warning)
	}
	if err = e.checkSession(); err != nil {
		return Result{}, err
	}
	lock, err := e.store().Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	if state.WorkStatus() == ledger.WorkActive {
		if _, id, paused := state.Paused(); paused {
			if _, err = e.append(&ledger.Resume{PauseID: id}); err != nil {
				return Result{}, err
			}
		}
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}

// Abort ends work without discarding its records or working tree.
func (e *Engine) Abort(ctx context.Context) (Result, error) {
	_, warning, err := e.resolve(ctx, false)
	if err != nil {
		return Result{}, err
	}
	if warning != "" {
		return Result{}, errors.New(warning)
	}
	lock, err := e.store().Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	if state.WorkStatus() == ledger.WorkActive {
		if _, err = e.append(&ledger.StateChange{WorkStatus: ledger.WorkAborted}); err != nil {
			return Result{}, err
		}
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	return e.terminal(ctx, state)
}

// Note accepts schema-validated decisions, evidence, and lessons without requiring a matching branch.
func (e *Engine) Note(ctx context.Context, kind ledger.Kind, data []byte) (Result, error) {
	_, _, err := e.resolve(ctx, false)
	if err != nil {
		return Result{}, err
	}
	lock, err := e.store().Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	var payload ledger.Payload
	switch kind {
	case ledger.KindDecision:
		payload = &ledger.Decision{}
	case ledger.KindEvidence:
		payload = &ledger.Evidence{}
	case ledger.KindLesson:
		payload = &ledger.Lesson{}
	default:
		return Result{}, fmt.Errorf("unsupported note kind: %q", kind)
	}
	if err = fsstore.DecodePayload(data, payload); err != nil {
		return Result{}, err
	}
	if _, err = e.append(payload); err != nil {
		return Result{}, err
	}
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}

// Plan records a human-initiated proposal; schema and version validation precede acceptance.
func (e *Engine) Plan(ctx context.Context, data []byte) (Result, error) {
	lock, err := e.mutation(ctx)
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	p, err := e.plans.ParsePlan(data)
	if err != nil {
		return Result{}, err
	}
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	if err = e.acceptPlan(ctx, state, p, plan.HumanProposal, ""); err != nil {
		return Result{}, err
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}

// WithdrawPlan withdraws the latest proposal. Only voluntary human proposals allow fallback.
func (e *Engine) WithdrawPlan(ctx context.Context) (Result, error) {
	lock, err := e.mutation(ctx)
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	versions := state.PlanEvaluations(nil)
	if len(versions) == 0 {
		return Result{}, errors.New("there is no proposal to withdraw")
	}
	if _, err = e.append(&ledger.Withdrawal{PlanVersion: versions[len(versions)-1].Plan.Version}); err != nil {
		return Result{}, err
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}
func latestPlan(state *ledger.State) (*ledger.PlanVersion, bool) {
	p := state.CurrentPosition().Plan
	return p, p != nil
}
func (e *Engine) records() ([]ledger.Record, error) { return ledger.List(e.store(), e.validator) }
func (e *Engine) highestPlan() (int, error) {
	rs, err := e.records()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rs {
		if p, ok := r.Payload.(*ledger.PlanVersion); ok {
			n = max(n, p.Plan.Version)
		}
	}
	return n, nil
}
func (e *Engine) acceptPlan(ctx context.Context, state *ledger.State, p plan.Plan, origin plan.ProposalOrigin, instructionID string) error {
	n, err := e.highestPlan()
	if err != nil {
		return err
	}
	if p.Version != n+1 {
		return fmt.Errorf("plan version must be %d", n+1)
	}
	reference, err := e.workspace.Head(ctx)
	if err != nil {
		return err
	}
	if instructionID != "" {
		issued, readErr := e.readInstruction(instructionID)
		if readErr != nil {
			return readErr
		}
		reference = issued.Reference
		if p.Template.Name != issued.TemplateName {
			return errors.New("planner template does not match its instruction")
		}
	}
	if !localName(p.Template.Name) {
		return errors.New("invalid plan template name")
	}
	source, err := e.workspace.ReadFile(ctx, reference, ".devkit/plans/"+p.Template.Name+".json")
	if err != nil {
		return err
	}
	if p.Template.ContentHash != plan.ContentHash(source) {
		return errors.New("plan template content hash does not match")
	}
	_, err = e.append(&ledger.PlanVersion{Plan: p, Origin: origin, InstructionID: instructionID})
	return err
}

// External delegates an approved operation to the sole external write executor.
func (e *Engine) External(ctx context.Context, request external.Request) (Result, error) {
	lock, err := e.mutation(ctx)
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	ref, err := e.reference(ctx, state)
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return Result{}, err
	}
	request.Plan = nil
	if p, ok := state.CurrentPosition().Plan, state.CurrentPosition().Plan != nil; ok {
		template, _, err := e.template(ctx, ref, p.Plan.Template.Name)
		if err != nil {
			return Result{}, err
		}
		norms, err := e.norms(ctx, ref)
		if err != nil {
			return Result{}, err
		}
		if selected, ok := state.ExecutablePlan(template, norms); ok {
			request.Plan = &selected
		}
	}
	executor := external.New(e.Root(), e.store(), e.validator, settings.External)
	outcome, err := executor.Execute(ctx, request)
	var approval *external.ApprovalRequired
	if errors.As(err, &approval) {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalExternal, External: &approval.Target}, "external_operation", "Approve this external write?", "")
		if q, err = e.saveQuestion(state, q, ""); err != nil {
			return Result{}, err
		}
		state, err = e.load()
		if err != nil {
			return Result{}, err
		}
		return e.issueQuestion(state, q)
	}
	if err != nil {
		return Result{}, err
	}
	if outcome.ConfirmationRequired {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalExternal, External: &outcome.Operation.ExternalOperationTarget}, "external_unknown", "The external outcome is unknown. Confirm its outcome before continuing.", "")
		if q, err = e.saveQuestion(state, q, ""); err != nil {
			return Result{}, err
		}
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	r := response(state)
	r.External = &outcome
	return r, nil
}

func (e *Engine) template(ctx context.Context, ref, name string) (plan.PlanTemplate, []byte, error) {
	if !localName(name) {
		return plan.PlanTemplate{}, nil, errors.New("invalid template name")
	}
	source, err := e.workspace.ReadFile(ctx, ref, ".devkit/plans/"+name+".json")
	if err != nil {
		return plan.PlanTemplate{}, nil, err
	}
	t, err := e.plans.ParseTemplate(source)
	return t, source, err
}
func (e *Engine) reference(ctx context.Context, state *ledger.State) (string, error) {
	if r, ok := state.OpenRound(); ok {
		return r.StartTarget.Tree, nil
	}
	return e.workspace.Head(ctx)
}
func (e *Engine) settings(ctx context.Context, ref string) (config.Config, error) {
	data, err := e.workspace.ReadFile(ctx, ref, "devkit.toml")
	if err != nil {
		return config.Config{}, err
	}
	return config.Parse(data)
}
func (e *Engine) norms(ctx context.Context, ref string) ([]form.NormReference, error) {
	ids, err := e.workspace.NormativeIDs(ctx, ref)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(ids))
	for path := range ids {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	norms := []form.NormReference{}
	for _, path := range paths {
		norm := form.NormReference{Path: path}
		if path == ".devkit" || strings.HasSuffix(path, "/agents") {
			norm.TreeID = ids[path]
		} else {
			norm.BlobID = ids[path]
		}
		norms = append(norms, norm)
	}
	return norms, nil
}
