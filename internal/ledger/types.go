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
const ReadableKitVersion = "v2"

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
	KindUnitResult    Kind = "unit_result"
	KindMerge         Kind = "merge"
	KindSourceWrite   Kind = "source_write"
)

const KindResultRejection Kind = "result_rejection"

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
	ID              string  `json:"id"`
	TaskRef         string  `json:"task_ref"`
	OriginalRequest *string `json:"original_request"`
	Branch          string  `json:"branch"`
	BaseCommit      string  `json:"base_commit"`
	KitVersion      string  `json:"kit_version"`
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
	AnswerBack     Interpretation = "back"
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

// UnitResult is a hook-observed worker termination block accepted for one unit.
type UnitResult struct {
	form.Progress
	RoundID    string `json:"round_id"`
	UnitID     string `json:"unit_id"`
	AgentID    string `json:"agent_id"`
	ResultPath string `json:"result_path"`
}

// MergeInput fixes one unit's working tree as an unbranched commit.
type MergeInput struct {
	InstructionID string `json:"instruction_id"`
	UnitID        string `json:"unit_id"`
	Commit        string `json:"commit"`
}

// Merge records the ordered inputs and each recoverable merge step without moving HEAD.
type Merge struct {
	RoundID    string       `json:"round_id"`
	Head       string       `json:"head"`
	Inputs     []MergeInput `json:"inputs"`
	Applied    int          `json:"applied"`
	Current    string       `json:"current"`
	BeforeTree string       `json:"before_tree"`
	ResultTree string       `json:"result_tree,omitempty"`
	Conflicts  []string     `json:"conflicts,omitempty"`
	Status     string       `json:"status"`
}

type Judgment struct {
	check.Judgment
	RoundID string `json:"round_id,omitempty"`
}

// ResultRejection records a malformed subagent result that must not be read again.
type ResultRejection struct {
	InstructionID string `json:"instruction_id"`
	ResultPath    string `json:"result_path"`
	Reason        string `json:"reason"`
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
	StartCommit    string               `json:"start_commit,omitempty"`
	MergeFix       bool                 `json:"merge_fix,omitempty"`
	StageRunID     string               `json:"stage_run_id"`
	SessionID      string               `json:"session_id"`
	StartTarget    form.TargetVersion   `json:"start_target"`
	ReturnedTarget *form.TargetVersion  `json:"returned_target,omitempty"`
	Norms          []form.NormReference `json:"norms"`
	NormReference  string               `json:"norm_reference,omitempty"`
	Decision       *router.Decision     `json:"decision,omitempty"`
}

// Instruction stores issuance facts; engine regenerates its text and brief.
type Instruction struct {
	ID           string                `json:"id"`
	Unit         *plan.Unit            `json:"unit,omitempty"`
	Kind         form.InstructionKind  `json:"kind"`
	TargetKey    string                `json:"target_key"`
	Reissue      int                   `json:"reissue"`
	SessionID    string                `json:"session_id"`
	RoundID      string                `json:"round_id,omitempty"`
	StageRunID   string                `json:"stage_run_id,omitempty"`
	PlanVersion  int                   `json:"plan_version,omitempty"`
	QuestionID   string                `json:"question_id,omitempty"`
	Reference    string                `json:"reference,omitempty"`
	Runner       string                `json:"runner,omitempty"`
	TargetPath   string                `json:"target_path,omitempty"`
	TemplateName string                `json:"template_name,omitempty"`
	Origin       plan.ProposalOrigin   `json:"origin,omitempty"`
	Request      *check.Request        `json:"request,omitempty"`
	Detection    *InstructionDetection `json:"detection,omitempty"`
	Context      *InstructionContext   `json:"context,omitempty"`
}

// InstructionDetection preserves the inputs used to accept a detection result.
// Check keeps the criteria and examples selections that produced the identity.
type InstructionDetection struct {
	Check    plan.Check              `json:"check"`
	Spec     check.DetectionSpec     `json:"spec"`
	Identity check.DetectionIdentity `json:"identity"`
	Examples map[string]string       `json:"examples"`
}

// InstructionOutput preserves a prior output supplied when an instruction was issued.
type InstructionOutput struct {
	StageID string `json:"stage_id"`
	Content string `json:"content"`
}

// InstructionContext keeps issuance-time facts, without storing rendered briefs.
type InstructionContext struct {
	Outputs           []InstructionOutput `json:"outputs"`
	Findings          []check.Finding     `json:"findings"`
	CurrentPlan       *plan.Plan          `json:"current_plan,omitempty"`
	RemainingPlan     *plan.Plan          `json:"remaining_plan,omitempty"`
	HumanInstructions string              `json:"human_instructions,omitempty"`
	Commands          []check.Judgment    `json:"commands,omitempty"`
	PreCommitOutput   string              `json:"pre_commit_output,omitempty"`
	WorkHistory       string              `json:"work_history,omitempty"`
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

func (*ResultRejection) recordKind() Kind { return KindResultRejection }
func (*UnitResult) recordKind() Kind      { return KindUnitResult }
func (*Merge) recordKind() Kind           { return KindMerge }
func (*SourceWrite) recordKind() Kind     { return KindSourceWrite }

// SourceWriteStatus is the state of one write to a knowledge source outside the repository.
type SourceWriteStatus string

const (
	SourceWriteRunning  SourceWriteStatus = "running"
	SourceWriteApplied  SourceWriteStatus = "applied"
	SourceWriteConflict SourceWriteStatus = "conflict"
)

// SourceWrite records a write of an approved document change through a source adapter.
// A running record precedes the write; a later record with the same operation ID gives its
// outcome. ExpectedHash is empty when the document must not exist, and ContentHash is empty
// for a deletion. ApprovalID refers to the answer that approved the target, when one exists.
type SourceWrite struct {
	OperationID  string             `json:"operation_id"`
	RoundID      string             `json:"round_id"`
	Source       string             `json:"source"`
	Document     string             `json:"document"`
	ExpectedHash string             `json:"expected_hash"`
	ContentHash  string             `json:"content_hash"`
	Target       form.TargetVersion `json:"target"`
	ApprovalID   string             `json:"approval_id,omitempty"`
	Status       SourceWriteStatus  `json:"status"`
	BeforeID     string             `json:"before_id,omitempty"`
}
