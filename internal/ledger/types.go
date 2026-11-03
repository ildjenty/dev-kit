package ledger

import (
	"encoding/json"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

// ReadableKitVersion identifies the record format understood by this binary.
const ReadableKitVersion = "v1"

type Kind string

const (
	KindWork          Kind = "work"
	KindPause         Kind = "pause"
	KindResume        Kind = "resume"
	KindPlan          Kind = "plan"
	KindWithdrawal    Kind = "withdrawal"
	KindDecision      Kind = "decision"
	KindEvidence      Kind = "evidence"
	KindLesson        Kind = "lesson"
	KindStateChange   Kind = "state_change"
	KindQuestion      Kind = "question"
	KindAnswer        Kind = "answer"
	KindApproval      Kind = "approval"
	KindExternalOp    Kind = "external_op"
	KindProgress      Kind = "progress"
	KindJudgment      Kind = "judgment"
	KindDetectionTest Kind = "detection_test"
	KindStageRun      Kind = "stage_run"
	KindRound         Kind = "round"
	KindInstruction   Kind = "instruction"
	KindArtifact      Kind = "artifact"
)

// Payload is a record payload. Only the record types in this package implement it.
// Use pointers to payloads when constructing records.
type Payload interface {
	recordKind() Kind
}

// Record belongs to the work identified by the supplied store root.
type Record struct {
	ID      string
	Payload Payload
}

func (r Record) Kind() Kind { return r.Payload.recordKind() }

type Work struct {
	ID         string `json:"id"`
	TaskRef    string `json:"task_ref"`
	Branch     string `json:"branch"`
	BaseCommit string `json:"base_commit"`
	KitVersion string `json:"kit_version"`
}

// Pause preserves a reference to the location before interruption.
type Pause struct {
	Position form.Reference `json:"position"`
	Reason   string         `json:"reason,omitempty"`
}

type Resume struct {
	PauseID string `json:"pause_id"`
}

// PlanVersion records a proposal, not its derived approval or execution status.
type PlanVersion struct {
	Plan          plan.Plan           `json:"plan"`
	Origin        plan.ProposalOrigin `json:"origin"`
	InstructionID string              `json:"instruction_id,omitempty"`
}

type Withdrawal struct {
	PlanVersion int `json:"plan_version"`
}

type Decision struct{ form.Decision }
type Evidence struct{ form.Evidence }

type Lesson struct {
	Content  string           `json:"content"`
	Evidence []form.Reference `json:"evidence"`
}

type DecisionStatus string

const (
	DecisionPending  DecisionStatus = "pending"
	DecisionApproved DecisionStatus = "approved"
	DecisionRejected DecisionStatus = "rejected"
	DecisionDeferred DecisionStatus = "deferred"
)

type WorkStatus string

const (
	WorkActive    WorkStatus = "active"
	WorkCompleted WorkStatus = "completed"
	WorkAborted   WorkStatus = "aborted"
)

// StateChange changes either a decision status or the terminal work status.
// Decision changes refer to the accepted answer that caused them.
type StateChange struct {
	DecisionID     string         `json:"decision_id,omitempty"`
	DecisionStatus DecisionStatus `json:"decision_status,omitempty"`
	AnswerID       string         `json:"answer_id,omitempty"`
	WorkStatus     WorkStatus     `json:"work_status,omitempty"`
}

type Question struct {
	form.Question
	Supersedes string `json:"supersedes,omitempty"`
}

type Interpretation string

const (
	AnswerApproved Interpretation = "approved"
	AnswerRejected Interpretation = "rejected"
	AnswerDeferred Interpretation = "deferred"
	AnswerText     Interpretation = "text"
)

// Answer keeps the original input after its observation file is deleted.
// Supersedes is the record ID of a corrected answer or approval.
type Answer struct {
	QuestionID     string              `json:"question_id"`
	Target         form.ApprovalTarget `json:"target"`
	Interpretation Interpretation      `json:"interpretation"`
	Text           string              `json:"text"`
	Original       string              `json:"original"`
	SessionID      string              `json:"session_id"`
	ObservedAt     time.Time           `json:"observed_at"`
	Supersedes     string              `json:"supersedes,omitempty"`
	InstructionID  string              `json:"instruction_id,omitempty"`
}

type Approval struct{ Answer }

type ExternalStatus string

const (
	ExternalRunning   ExternalStatus = "running"
	ExternalSucceeded ExternalStatus = "succeeded"
	ExternalFailed    ExternalStatus = "failed"
	ExternalUnknown   ExternalStatus = "unknown"
)

// ExternalOp is appended before execution and again when a result is known.
type ExternalOp struct {
	form.ExternalOperationTarget
	ApprovalID    string          `json:"approval_id"`
	BeforeID      string          `json:"before_id,omitempty"`
	Input         json.RawMessage `json:"input"`
	Status        ExternalStatus  `json:"status"`
	Result        string          `json:"result,omitempty"`
	ExternalID    string          `json:"external_id,omitempty"`
	InstructionID string          `json:"instruction_id,omitempty"`
}

type Progress struct {
	form.Progress
	RoundID string `json:"round_id"`
}

type Judgment struct {
	check.Judgment
	RoundID string `json:"round_id,omitempty"`
}

type DetectionTest struct {
	check.DetectionTest
	InstructionID string `json:"instruction_id"`
}

type StageState string

const (
	StageWorking      StageState = "working"
	StageJudgmentWait StageState = "judgment_wait"
	StageAnswerWait   StageState = "answer_wait"
	StageApprovalWait StageState = "approval_wait"
	StageExternalWait StageState = "external_wait"
	StageInvalidated  StageState = "invalidated"
	StageCompleted    StageState = "completed"
)

// StageRun updates are appended with the same ID, which is the initial record ID.
// The plan version remains bound for the lifetime of the run.
type StageRun struct {
	ID          string     `json:"id"`
	StageID     string     `json:"stage_id"`
	PlanVersion int        `json:"plan_version"`
	State       StageState `json:"state"`
	StopReason  string     `json:"stop_reason,omitempty"`
	NextActor   string     `json:"next_actor,omitempty"`
	Commit      string     `json:"commit,omitempty"`
}

// Round updates retain the initial record ID. Decision closes the round.
type Round struct {
	ID             string               `json:"id"`
	StageRunID     string               `json:"stage_run_id"`
	SessionID      string               `json:"session_id"`
	StartTarget    form.TargetVersion   `json:"start_target"`
	ReturnedTarget *form.TargetVersion  `json:"returned_target,omitempty"`
	Norms          []form.NormReference `json:"norms"`
	Decision       *router.Decision     `json:"decision,omitempty"`
}

// Instruction stores issuance metadata only; engine regenerates its text and brief.
type Instruction struct {
	ID          string               `json:"id"`
	Kind        form.InstructionKind `json:"kind"`
	TargetKey   string               `json:"target_key"`
	Reissue     int                  `json:"reissue"`
	SessionID   string               `json:"session_id"`
	RoundID     string               `json:"round_id,omitempty"`
	StageRunID  string               `json:"stage_run_id,omitempty"`
	PlanVersion int                  `json:"plan_version,omitempty"`
	QuestionID  string               `json:"question_id,omitempty"`
}

type Artifact struct {
	Before                 form.TargetVersion `json:"before"`
	After                  form.TargetVersion `json:"after"`
	CompletionConditionIDs []string           `json:"completion_condition_ids"`
	ChangeKind             string             `json:"change_kind"`
	Decisions              []form.Reference   `json:"decisions"`
	Commit                 string             `json:"commit,omitempty"`
}

// StopObservation is a generated input supplied by hook or cmd, never a ledger record.
type StopObservation struct {
	ID            string         `json:"id"`
	WorkID        string         `json:"work_id"`
	SessionID     string         `json:"session_id"`
	ObservedAt    time.Time      `json:"observed_at"`
	LastRecordID  string         `json:"last_record_id"`
	Continuations int            `json:"continuations"`
	LimitReached  bool           `json:"limit_reached"`
	Question      *form.Question `json:"question,omitempty"`
}

func (*Work) recordKind() Kind          { return KindWork }
func (*Pause) recordKind() Kind         { return KindPause }
func (*Resume) recordKind() Kind        { return KindResume }
func (*PlanVersion) recordKind() Kind   { return KindPlan }
func (*Withdrawal) recordKind() Kind    { return KindWithdrawal }
func (*Decision) recordKind() Kind      { return KindDecision }
func (*Evidence) recordKind() Kind      { return KindEvidence }
func (*Lesson) recordKind() Kind        { return KindLesson }
func (*StateChange) recordKind() Kind   { return KindStateChange }
func (*Question) recordKind() Kind      { return KindQuestion }
func (*Answer) recordKind() Kind        { return KindAnswer }
func (*Approval) recordKind() Kind      { return KindApproval }
func (*ExternalOp) recordKind() Kind    { return KindExternalOp }
func (*Progress) recordKind() Kind      { return KindProgress }
func (*Judgment) recordKind() Kind      { return KindJudgment }
func (*DetectionTest) recordKind() Kind { return KindDetectionTest }
func (*StageRun) recordKind() Kind      { return KindStageRun }
func (*Round) recordKind() Kind         { return KindRound }
func (*Instruction) recordKind() Kind   { return KindInstruction }
func (*Artifact) recordKind() Kind      { return KindArtifact }
