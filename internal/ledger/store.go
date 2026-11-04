package ledger

import (
	"fmt"
	"reflect"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/oklog/ulid/v2"
)

// NewRecord assigns an ID greater than every stored record ID.
// The caller holds the work lock through creation and Append, except for pause.
// Entity IDs for new stage runs and rounds use this ID.
func NewRecord(store *fsstore.Store, payload Payload) (Record, error) {
	id, err := store.NextID()
	if err != nil {
		return Record{}, err
	}
	return Record{ID: id, Payload: payload}, nil
}

// Append validates and publishes a record. The caller owns the work lock lifetime.
func Append(store *fsstore.Store, record Record, validator *Validator) error {
	if err := validator.validate(record); err != nil {
		return fmt.Errorf("record %s.json: %w", record.ID, err)
	}
	for _, ref := range references(record) {
		if ref.RecordID != "" {
			referenced, err := Get(store, ref.RecordID, validator)
			if err == nil {
				err = validateDecisionAnswer(record, referenced)
			}
			if err != nil {
				return fmt.Errorf("record %s.json: reference %s: %w", record.ID, ref.RecordID, err)
			}
		} else if ref.External == "" {
			return fmt.Errorf("record %s.json: reference must identify a record or external source", record.ID)
		}
		if ref.RecordID != "" && ref.External != "" {
			return fmt.Errorf("record %s.json: reference cannot identify both a record and an external source", record.ID)
		}
	}
	return store.AppendRecord(record.ID, string(record.Kind()), record.Payload)
}

// Get reads a typed record. Corrupt and incompatible records are left in place.
func Get(store *fsstore.Store, id string, validator *Validator) (Record, error) {
	raw, err := store.GetRecord(id)
	if err != nil {
		return Record{}, err
	}
	record, err := readRecord(raw, validator)
	if err != nil {
		return Record{}, err
	}
	for _, ref := range references(record) {
		if ref.RecordID == "" {
			continue
		}
		raw, err := store.GetRecord(ref.RecordID)
		if err == nil {
			var referenced Record
			referenced, err = readRecord(raw, validator)
			if err == nil {
				err = validateDecisionAnswer(record, referenced)
			}
		}
		if err != nil {
			return Record{}, fmt.Errorf("record %s.json: reference %s: %w", record.ID, ref.RecordID, err)
		}
	}
	return record, nil
}

// List returns typed records in fsstore's ID order, stopping at unreadable records.
func List(store *fsstore.Store, validator *Validator) ([]Record, error) {
	stored, err := store.List()
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(stored))
	byID := make(map[string]Record, len(stored))
	for _, raw := range stored {
		record, err := readRecord(raw, validator)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
		byID[record.ID] = record
	}
	for _, record := range records {
		for _, ref := range references(record) {
			if referenced, exists := byID[ref.RecordID]; ref.RecordID != "" && exists && ref.External == "" {
				if err := validateDecisionAnswer(record, referenced); err != nil {
					return nil, fmt.Errorf("record %s.json: reference %s: %w", record.ID, ref.RecordID, err)
				}
				continue
			}
			if ref.RecordID == "" && ref.External != "" {
				continue
			}
			return nil, fmt.Errorf("record %s.json: invalid or missing reference: %+v", record.ID, ref)
		}
	}
	return records, nil
}

func readRecord(raw fsstore.Record, validator *Validator) (Record, error) {
	var payload Payload
	switch Kind(raw.Kind) {
	case KindWork:
		payload = &Work{}
	case KindPause:
		payload = &Pause{}
	case KindResume:
		payload = &Resume{}
	case KindPlan:
		payload = &PlanVersion{}
	case KindWithdrawal:
		payload = &Withdrawal{}
	case KindDecision:
		payload = &Decision{}
	case KindEvidence:
		payload = &Evidence{}
	case KindLesson:
		payload = &Lesson{}
	case KindStateChange:
		payload = &StateChange{}
	case KindQuestion:
		payload = &Question{}
	case KindAnswer:
		payload = &Answer{}
	case KindApproval:
		payload = &Approval{}
	case KindExternalOp:
		payload = &ExternalOp{}
	case KindProgress:
		payload = &Progress{}
	case KindJudgment:
		payload = &Judgment{}
	case KindDetectionTest:
		payload = &DetectionTest{}
	case KindStageRun:
		payload = &StageRun{}
	case KindRound:
		payload = &Round{}
	case KindInstruction:
		payload = &Instruction{}
	case KindArtifact:
		payload = &Artifact{}
	default:
		return Record{}, fmt.Errorf("record %s.json: unknown record kind: %q", raw.ID, raw.Kind)
	}
	if err := fsstore.DecodePayload(raw.Payload, payload); err != nil {
		return Record{}, fmt.Errorf("record %s.json: %w", raw.ID, err)
	}
	record := Record{ID: raw.ID, Payload: payload}
	if err := validator.validateRaw(payload, raw.Payload); err != nil {
		return Record{}, fmt.Errorf("record %s.json: %w", raw.ID, err)
	}
	if err := validator.validate(record); err != nil {
		return Record{}, fmt.Errorf("record %s.json: %w", raw.ID, err)
	}
	return record, nil
}

func (v *Validator) validate(r Record) error {
	if _, err := ulid.ParseStrict(r.ID); err != nil {
		return fmt.Errorf("record ID is not a ULID: %w", err)
	}
	if r.Payload == nil || reflect.ValueOf(r.Payload).IsNil() {
		return fmt.Errorf("record payload is required")
	}
	switch p := r.Payload.(type) {
	case *Work:
		if p.KitVersion != ReadableKitVersion {
			return fmt.Errorf("unreadable kit version %q; readable version is %q", p.KitVersion, ReadableKitVersion)
		}
		if p.ID == "" || p.TaskRef == "" || p.Branch == "" || p.BaseCommit == "" {
			return fmt.Errorf("work identity, task reference, branch, and base commit are required")
		}
	case *StageRun:
		if p.ID == "" || p.StageID == "" || p.PlanVersion <= 0 {
			return fmt.Errorf("stage run identity, stage, and plan version are required")
		}
		switch p.State {
		case StageWorking, StageJudgmentWait, StageAnswerWait, StageApprovalWait, StageExternalWait, StageInvalidated:
		case StageCompleted:
			if p.Commit == "" {
				return fmt.Errorf("completed stage run requires a commit")
			}
		default:
			return fmt.Errorf("unknown stage state: %q", p.State)
		}
	case *Round:
		if p.ID == "" || p.StageRunID == "" || p.SessionID == "" {
			return fmt.Errorf("round identity, stage run, and session are required")
		}
	case *PlanVersion:
		if p.Plan.Version <= 0 {
			return fmt.Errorf("plan version must be positive")
		}
	case *StateChange:
		if err := validateStateChange(p); err != nil {
			return err
		}
	}
	for _, ref := range references(r) {
		if err := v.check("reference", ref); err != nil {
			return err
		}
	}
	return v.validatePayload(r.Payload)
}

func references(r Record) []form.Reference {
	refs := []form.Reference{}
	add := func(id string) {
		if id != "" {
			refs = append(refs, form.Reference{RecordID: id})
		}
	}
	answer := func(a Answer) { add(a.Supersedes) }
	question := func(q form.Question) {
		refs = append(refs, q.Evidence...)
		if q.StopObservation != nil {
			refs = append(refs, *q.StopObservation)
		}
	}
	switch p := r.Payload.(type) {
	case *Pause:
		refs = append(refs, p.Position)
	case *Resume:
		add(p.PauseID)
	case *Decision:
		refs = append(refs, p.Supersedes...)
		refs = append(refs, p.DependsOn...)
		refs = append(refs, p.Evidence...)
	case *Evidence:
		refs = append(refs, p.Sources...)
	case *Lesson:
		refs = append(refs, p.Evidence...)
	case *StateChange:
		add(p.AnswerID)
	case *Question:
		question(p.Question)
	case *Answer:
		answer(*p)
	case *Approval:
		answer(p.Answer)
	case *ExternalOp:
		add(p.ApprovalID)
		add(p.BeforeID)
	case *Progress:
		add(p.RoundID)
	case *Judgment:
		add(p.RoundID)
		for _, finding := range p.Findings {
			if finding.Exemption != nil {
				refs = append(refs, finding.Exemption.Evidence...)
			}
		}
	case *StageRun:
		if p.ID != r.ID {
			add(p.ID)
		}
	case *Round:
		add(p.StageRunID)
		if p.ID != r.ID {
			add(p.ID)
		}
	case *Instruction:
		add(p.RoundID)
		add(p.StageRunID)
	case *Artifact:
		refs = append(refs, p.Decisions...)
	}
	return refs
}

func validateStateChange(p *StateChange) error {
	if p.WorkStatus != "" {
		if p.DecisionID != "" || p.DecisionStatus != "" || p.AnswerID != "" || p.WorkStatus != WorkCompleted && p.WorkStatus != WorkAborted {
			return fmt.Errorf("invalid work state change")
		}
	} else if p.DecisionID == "" || p.AnswerID == "" || p.DecisionStatus != DecisionApproved && p.DecisionStatus != DecisionRejected && p.DecisionStatus != DecisionDeferred {
		return fmt.Errorf("invalid decision state change")
	}
	return nil
}
