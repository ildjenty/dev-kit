package ledger

import (
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Position combines the current execution location with the independent pause overlay.
type Position struct {
	Work         *Work          `json:"work,omitempty"`
	Status       WorkStatus     `json:"status"`
	StageRun     *StageRun      `json:"stage_run,omitempty"`
	Round        *Round         `json:"round,omitempty"`
	Plan         *PlanVersion   `json:"plan,omitempty"`
	ExternalOps  []*ExternalOp  `json:"external_ops"`
	Paused       bool           `json:"paused"`
	PauseID      string         `json:"pause_id,omitempty"`
	BeforePause  form.Reference `json:"before_pause"`
	LastRecordID string         `json:"last_record_id,omitempty"`
}

func (s *State) CurrentPosition() Position {
	position := Position{Status: s.WorkStatus(), LastRecordID: s.LastRecordID(), ExternalOps: s.ExternalOperations()}
	position.Work, _ = s.Work()
	position.StageRun, _ = s.LatestStageRun()
	position.Round, _ = s.OpenRound()
	for i := len(s.records) - 1; i >= 0; i-- {
		if proposal, ok := s.records[i].Payload.(*PlanVersion); ok {
			if position.StageRun != nil && position.StageRun.State != StageCompleted && position.StageRun.State != StageInvalidated && proposal.Plan.Version != position.StageRun.PlanVersion {
				continue
			}
			position.Plan = proposal
			break
		}
	}
	if pause, id, paused := s.Paused(); paused {
		position.Paused, position.PauseID, position.BeforePause = true, id, pause.Position
	}
	return position
}

// UnacceptedInstructions returns the current issuance per kind and target key.
// Reissued, answered, completed, invalidated, and withdrawn targets are omitted.
func (s *State) UnacceptedInstructions() []Instruction {
	accepted := make(map[string]bool)
	withdrawn := make(map[int]bool)
	latestPlan := 0
	for _, record := range s.records {
		switch p := record.Payload.(type) {
		case *Progress:
			accepted[p.InstructionID] = true
		case *Judgment:
			accepted[p.InstructionID] = true
		case *PlanVersion:
			accepted[p.InstructionID] = true
		case *DetectionTest:
			accepted[p.InstructionID] = true
		case *Answer:
			accepted[p.InstructionID] = true
		case *Approval:
			accepted[p.InstructionID] = true
		case *ExternalOp:
			accepted[p.InstructionID] = true
		case *Withdrawal:
			withdrawn[p.PlanVersion] = true
		}
	}
	for _, record := range s.records {
		if proposal, ok := record.Payload.(*PlanVersion); ok && !(withdrawn[proposal.Plan.Version] && proposal.Origin == plan.HumanProposal) {
			latestPlan = max(latestPlan, proposal.Plan.Version)
		}
	}

	type key struct {
		kind   form.InstructionKind
		target string
	}
	latest := []Instruction{}
	indices := make(map[key]int)
	for _, record := range s.records {
		if instruction, ok := record.Payload.(*Instruction); ok {
			k := key{instruction.Kind, instruction.TargetKey}
			if index, exists := indices[k]; exists {
				latest[index] = *instruction
			} else {
				indices[k] = len(latest)
				latest = append(latest, *instruction)
			}
		}
	}
	runs := make(map[string]*StageRun)
	rounds := make(map[string]*Round)
	for _, run := range s.StageRuns() {
		runs[run.ID] = run
		for _, round := range s.Rounds(run.ID) {
			rounds[round.ID] = round
		}
	}
	pendingQuestions := make(map[string]bool)
	for _, question := range s.UnansweredQuestions() {
		pendingQuestions[question.ID] = true
	}
	pending := []Instruction{}
	for _, instruction := range latest {
		if accepted[instruction.ID] || instruction.Kind == form.InstructionCompleted || instruction.Kind == form.InstructionAborted {
			continue
		}
		if instruction.PlanVersion != 0 && (withdrawn[instruction.PlanVersion] || instruction.PlanVersion < latestPlan && instruction.RoundID == "" && instruction.StageRunID == "") {
			continue
		}
		if run := runs[instruction.StageRunID]; run != nil && (run.State == StageCompleted || run.State == StageInvalidated) {
			continue
		}
		if round := rounds[instruction.RoundID]; round != nil {
			run := runs[round.StageRunID]
			if round.Decision != nil || run != nil && (run.State == StageCompleted || run.State == StageInvalidated) {
				continue
			}
		}
		if instruction.QuestionID != "" && !pendingQuestions[instruction.QuestionID] {
			continue
		}
		if s.WorkStatus() == WorkActive {
			pending = append(pending, instruction)
		}
	}
	return pending
}

// Checkpoint is derived for each injection and is never appended as a record.
type Checkpoint struct {
	Position     Position           `json:"position"`
	Progress     *Progress          `json:"progress,omitempty"`
	Decisions    []ResolvedDecision `json:"decisions"`
	Questions    []form.Question    `json:"questions"`
	Instructions []Instruction      `json:"instructions"`
}

// Checkpoint preserves temporary state and its conditions without evaluating them.
func (s *State) Checkpoint() (Checkpoint, error) {
	decisions, err := s.ResolveDecisions(DecisionContext{})
	if err != nil {
		return Checkpoint{}, err
	}
	checkpoint := Checkpoint{
		Position: s.CurrentPosition(), Decisions: decisions, Questions: s.UnansweredQuestions(), Instructions: s.UnacceptedInstructions(),
	}
	if progress, ok := s.LatestProgress(); ok {
		copy := *progress
		copy.TemporaryState = append([]form.TemporaryState{}, progress.TemporaryState...)
		checkpoint.Progress = &copy
	}
	return checkpoint, nil
}

// BuildCheckpoint loads a fresh snapshot before assembling its checkpoint.
func BuildCheckpoint(store *fsstore.Store, stops []StopObservation, validator *Validator) (Checkpoint, error) {
	state, err := Load(store, stops, validator)
	if err != nil {
		return Checkpoint{}, err
	}
	return state.Checkpoint()
}
