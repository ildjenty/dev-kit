package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/oklog/ulid/v2"
)

// AgentDefinition contains launch settings read from the normative agent definition.
type AgentDefinition struct {
	Name   string
	Model  string
	Effort string
}

// Handler processes common host events. Root is the resolved checkout root, and
// WorkID is the binding resolved by cmd/devkit from the session observations.
// ReadConfig and ReadAgent read the supplied git reference, never the working tree.
// InstructionRunner resolves the runner of an issued instruction from its request.
type Handler struct {
	Root              string
	WorkID            string
	Validator         *ledger.Validator
	ReadConfig        func(reference string) (config.Config, error)
	ReadAgent         func(reference string, host event.Host, name string) (AgentDefinition, error)
	InstructionRunner func(ledger.Instruction) (string, error)
}

// Process returns a result for host/event.Format. It never appends ledger records
// or acquires the work lock. SessionStart failures are injected as warnings.
func (h Handler) Process(e event.Event) (result event.Result, err error) {
	if e.AgentID != "" && (e.Name == event.SessionStart || e.Name == event.Stop) {
		return event.Result{}, nil
	}
	defer func() {
		if err != nil && e.Name == event.SessionStart {
			result = event.Result{AdditionalContext: "Warning: " + err.Error()}
			err = nil
		} else if err != nil && e.Name == event.PreToolUse {
			result = denied("Cannot evaluate tool use: " + err.Error())
			err = nil
		}
	}()
	now := time.Now().UTC()
	if e.Name == event.SessionStart || e.Name == event.PreToolUse && h.WorkID != "" {
		err = PublishSession(h.runRoot(), SessionObservation{
			Host: e.Host, SessionID: e.SessionID, ObservedAt: now, WorkID: h.WorkID,
		}, string(e.Name))
		if err != nil && !pauseCommand(e) {
			return event.Result{}, err
		}
	}
	if pauseCommand(e) {
		result.Decision = event.Allow
		if err != nil {
			result.SystemMessage = "Warning: " + err.Error()
		}
		return result, nil
	}
	if e.Name == event.UserPromptSubmit {
		if e.Host == event.ClaudeCode && strings.HasPrefix(e.Prompt, "<task-notification>") {
			return event.Result{}, nil
		}
		if err := fsstore.New(h.runRoot()).Publish(filepath.Join("sessions", e.SessionID, "latest-prompt.txt"), []byte(e.Prompt)); err != nil {
			return event.Result{}, err
		}
	}
	if h.WorkID == "" {
		return event.Result{}, nil
	}
	switch e.Name {
	case event.UserPromptSubmit:
		name := filepath.Join("prompts", e.SessionID+"-"+timestamp(now)+".txt")
		return event.Result{}, fsstore.New(h.workRoot()).Publish(name, []byte(e.Prompt))
	case event.SessionStart, event.PreToolUse, event.Stop, event.SubagentStop:
	default:
		return event.Result{}, fmt.Errorf("unsupported event: %q", e.Name)
	}
	if h.Validator == nil {
		return event.Result{}, fmt.Errorf("ledger validator is required")
	}
	state, err := ledger.Load(fsstore.New(h.workRoot()), nil, h.Validator)
	if err != nil {
		return event.Result{}, err
	}
	switch e.Name {
	case event.SessionStart:
		return h.sessionStart()
	case event.PreToolUse:
		return h.preToolUse(e, state)
	case event.SubagentStop:
		return event.Result{}, h.subagentStop(e, state, now)
	case event.Stop:
		return h.stop(e, state, now)
	}
	return event.Result{}, nil
}

func (h Handler) runRoot() string  { return filepath.Join(h.Root, ".devkit", "run") }
func (h Handler) workRoot() string { return filepath.Join(h.runRoot(), "work", h.WorkID) }

func (h Handler) subagentStop(e event.Event, state *ledger.State, now time.Time) error {
	if !strings.HasPrefix(e.AgentType, "devkit-") || e.AgentID == "" {
		return nil
	}
	var identity struct {
		InstructionID string `json:"instruction_id"`
	}
	if json.Unmarshal([]byte(e.LastAssistantMessage), &identity) != nil || identity.InstructionID == "" {
		return nil
	}
	for _, instruction := range state.UnacceptedInstructions() {
		if instruction.ID != identity.InstructionID || !subagentInstruction(instruction.Kind) {
			continue
		}
		if h.InstructionRunner == nil {
			return fmt.Errorf("instruction runner resolver is required")
		}
		runner, err := h.InstructionRunner(instruction)
		if err != nil {
			return err
		}
		if runner != e.AgentType {
			return nil
		}
		result := SubagentResult{
			InstructionID: instruction.ID, Runner: runner, AgentID: e.AgentID,
			Host: e.Host, ObservedAt: now, Content: json.RawMessage(e.LastAssistantMessage),
		}
		if e.Host == event.ClaudeCode {
			result.Effort = e.Effort
		} else if e.Host == event.Codex {
			result.Model = e.Model
		}
		return publishSubagentResult(h.workRoot(), result)
	}
	return nil
}

func subagentInstruction(kind form.InstructionKind) bool {
	switch kind {
	case form.InstructionJudgment, form.InstructionPlanCheck, form.InstructionDetectionTest, form.InstructionPlan:
		return true
	}
	return false
}

func timestamp(now time.Time) string { return now.UTC().Format("20060102T150405.000000000Z") }

func publishJSON(store *fsstore.Store, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return store.Publish(name, data)
}

func (h Handler) stop(e event.Event, state *ledger.State, now time.Time) (event.Result, error) {
	observation := ledger.StopObservation{
		ID: ulid.Make().String(), WorkID: h.WorkID, SessionID: e.SessionID,
		ObservedAt: now, LastRecordID: state.LastRecordID(),
	}
	result := event.Result{}
	if _, _, paused := state.Paused(); !paused {
		stops, err := ReadStops(h.workRoot())
		if err != nil {
			return result, err
		}
		state, err = ledger.Load(fsstore.New(h.workRoot()), stops, h.Validator)
		if err != nil {
			return result, err
		}
		// Use the same ledger snapshot for counting and deciding whether to stop.
		observation.LastRecordID = state.LastRecordID()
		if len(stops) != 0 {
			last := stops[len(stops)-1]
			if last.LastRecordID == observation.LastRecordID {
				observation.Continuations = last.Continuations
				observation.LimitReached = last.LimitReached
			}
		}
		result, err = h.stopDecision(state, &observation)
		if err != nil {
			return result, err
		}
	}
	if err := publishJSON(fsstore.New(h.workRoot()), filepath.Join("stops", observation.ID+".json"), observation); err != nil {
		return event.Result{}, err
	}
	return result, nil
}

func stopAllowed(state *ledger.State) bool {
	if state.WorkStatus() != ledger.WorkActive {
		return true
	}
	for _, instruction := range state.UnacceptedInstructions() {
		switch instruction.Kind {
		case form.InstructionQuestion, form.InstructionExternal, form.InstructionCompleted,
			form.InstructionAborted, form.InstructionJudgmentWait, form.InstructionJudgment,
			form.InstructionPlanCheck, form.InstructionDetectionTest:
			return true
		}
	}
	return false
}

func continuationQuestion(workID string, stop ledger.StopObservation, position ledger.Position) form.Question {
	question := form.Question{
		ID:             workID + "-stop-" + stop.LastRecordID,
		Text:           "No ledger records have been added within the continuation limit. Should this work continue?",
		ApprovalTarget: form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: workID},
		Options:        []string{"continue", "pause"}, Recommendation: "pause",
		RecommendationReason: "Check why the work has made no recorded progress before continuing.",
		StopCategory:         "continuation_limit", StopReason: "The work reached the continuation limit without new ledger records.",
		Evidence: []form.Reference{}, StopObservation: &form.Reference{External: "stops/" + stop.ID + ".json"},
	}
	if stop.LastRecordID != "" {
		question.Evidence = append(question.Evidence, form.Reference{RecordID: stop.LastRecordID})
	}
	if position.StageRun != nil {
		question.StoppedStage, question.NextStage = position.StageRun.StageID, position.StageRun.StageID
	}
	return question
}

func (h Handler) sessionStart() (event.Result, error) {
	stops, err := ReadStops(h.workRoot())
	if err != nil {
		return event.Result{}, err
	}
	checkpoint, err := ledger.BuildCheckpoint(fsstore.New(h.workRoot()), stops, h.Validator)
	if err != nil {
		return event.Result{}, err
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return event.Result{}, err
	}
	summary := fmt.Sprintf("Work %s: %s.", h.WorkID, checkpoint.Position.Status)
	if checkpoint.Position.Paused {
		summary += " Work is paused. Use `devkit resume` only when explicitly requested by the human."
	} else {
		summary += " Run `devkit next` for the current instruction."
	}
	return event.Result{AdditionalContext: summary + "\n\n```json\n" + string(data) + "\n```", Summary: summary}, nil
}

func (h Handler) stopDecision(state *ledger.State, observation *ledger.StopObservation) (event.Result, error) {
	result := event.Result{}
	if _, _, paused := state.Paused(); paused {
		return result, nil
	}
	var err error
	questions := state.UnansweredQuestions()
	if len(questions) != 0 {
		for _, question := range questions {
			if question.StopObservation != nil {
				result.SystemMessage, err = form.RenderMarkdown(question)
				if err != nil {
					return result, err
				}
				break
			}
		}
	} else if !stopAllowed(state) {
		observation.Continuations++
		limit, err := h.continuationLimit(state)
		if err != nil {
			return result, err
		}
		if observation.Continuations >= limit {
			observation.LimitReached = true
			question := continuationQuestion(h.WorkID, *observation, state.CurrentPosition())
			observation.Question = &question
			result.SystemMessage, err = form.RenderMarkdown(question)
			if err != nil {
				return result, err
			}
		} else {
			result.Decision, result.Reason = event.Block, "Run `devkit next`."
		}
	}
	return result, nil
}

func (h Handler) continuationLimit(state *ledger.State) (int, error) {
	limit := 3
	if h.ReadConfig != nil {
		work, _ := state.Work()
		ref := work.Branch
		if round, ok := state.OpenRound(); ok {
			ref = round.StartTarget.Tree
		}
		settings, err := h.ReadConfig(ref)
		if err != nil {
			return 0, err
		}
		limit = settings.StopContinuation()
	}
	return limit, nil
}
