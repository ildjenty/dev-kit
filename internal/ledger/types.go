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
const ReadableKitVersion = "v3"

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
	KindStageRun      Kind = "stage_run"
	KindRound         Kind = "round"
	KindInstruction   Kind = "instruction"
	KindArtifact      Kind = "artifact"
	KindUnitResult    Kind = "unit_result"
	KindMerge         Kind = "merge"
	KindJudgmentBrief Kind = "judgment_brief"
)

const KindResultRejection Kind = "result_rejection"

const KindMainInstruction Kind = "main_instruction"

// MainInstruction records verbatim guidance from the main agent for the current work.
type MainInstruction struct {
	Text     string `json:"text"`
	StageID  string `json:"stage_id,omitempty"`
	Audience string `json:"audience,omitempty"`
}

func (*MainInstruction) recordKind() Kind { return KindMainInstruction }

// Payload is a record payload. Only the record types in this package implement it.
// Use pointers to payloads when constructing records.
type Payload interface {
	recordKind() Kind
}

// Record belongs to the work identified by the supplied store root.
type Record struct {
	ID      string
	NoteKey string
	Payload Payload
}

func (r Record) Kind() Kind { return r.Payload.recordKind() }

// Work records the branch checked out at start only for mismatch warnings; the checkout identifies work.
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
	Repair     bool         `json:"repair,omitempty"`
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

// JudgmentBrief is a planner's supplementary instruction for one round, aspect, and runner.
// It cannot narrow the fixed criteria or the judgment scope.
type JudgmentBrief struct {
	InstructionID string   `json:"instruction_id"`
	RoundID       string   `json:"round_id"`
	Aspect        string   `json:"aspect"`
	Runner        string   `json:"runner"`
	Instructions  []string `json:"instructions"`
	AgentID       string   `json:"agent_id"`
	ResultPath    string   `json:"result_path"`
}

// ResultRejection records a malformed subagent result that must not be read again.
type ResultRejection struct {
	InstructionID string `json:"instruction_id"`
	ResultPath    string `json:"result_path"`
	Reason        string `json:"reason"`
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
	Assignee       plan.Assignee        `json:"assignee,omitempty"`
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
	ID          string               `json:"id"`
	Unit        *plan.Unit           `json:"unit,omitempty"`
	Kind        form.InstructionKind `json:"kind"`
	TargetKey   string               `json:"target_key"`
	Reissue     int                  `json:"reissue"`
	SessionID   string               `json:"session_id"`
	RoundID     string               `json:"round_id,omitempty"`
	StageRunID  string               `json:"stage_run_id,omitempty"`
	PlanVersion int                  `json:"plan_version,omitempty"`
	QuestionID  string               `json:"question_id,omitempty"`
	Reference   string               `json:"reference,omitempty"`
	Inputs      []plan.Selection     `json:"inputs,omitempty"`
	Runner      string               `json:"runner,omitempty"`
	TargetPath  string               `json:"target_path,omitempty"`
	PatternName string               `json:"pattern_name,omitempty"`
	Origin      plan.ProposalOrigin  `json:"origin,omitempty"`
	Request     *check.Request       `json:"request,omitempty"`
	Context     *InstructionContext  `json:"context,omitempty"`
}

// InstructionOutput preserves a prior output supplied when an instruction was issued.
type InstructionOutput struct {
	StageID    string `json:"stage_id"`
	Definition string `json:"definition,omitempty"`
	Content    string `json:"content"`
}

// InstructionContext keeps issuance-time facts, without storing rendered briefs.
type InstructionContext struct {
	MainInstructions  string              `json:"main_instructions,omitempty"`
	Outputs           []InstructionOutput `json:"outputs"`
	Findings          []check.Finding     `json:"findings"`
	CurrentPlan       *plan.Plan          `json:"current_plan,omitempty"`
	RemainingPlan     *plan.Plan          `json:"remaining_plan,omitempty"`
	HumanInstructions string              `json:"human_instructions,omitempty"`
	Commands          []check.Judgment    `json:"commands,omitempty"`
	PreCommitOutput   string              `json:"pre_commit_output,omitempty"`
	WorkHistory       string              `json:"work_history,omitempty"`
	CurrentMaterial   string              `json:"current_material,omitempty"`
	Schemas           []string            `json:"schemas,omitempty"`
	Instructions      []string            `json:"instructions,omitempty"`
	PreJudgment       []string            `json:"pre_judgment,omitempty"`
	Delegations       []InstructionOutput `json:"delegations,omitempty"`
	BriefBytes        int                 `json:"brief_bytes,omitempty"`
	Overflow          []string            `json:"overflow,omitempty"`
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

func (*Work) recordKind() Kind        { return KindWork }
func (*Pause) recordKind() Kind       { return KindPause }
func (*Resume) recordKind() Kind      { return KindResume }
func (*PlanVersion) recordKind() Kind { return KindPlan }
func (*Withdrawal) recordKind() Kind  { return KindWithdrawal }
func (*Decision) recordKind() Kind    { return KindDecision }
func (*Evidence) recordKind() Kind    { return KindEvidence }
func (*Lesson) recordKind() Kind      { return KindLesson }
func (*StateChange) recordKind() Kind { return KindStateChange }
func (*Question) recordKind() Kind    { return KindQuestion }
func (*Answer) recordKind() Kind      { return KindAnswer }
func (*Approval) recordKind() Kind    { return KindApproval }
func (*ExternalOp) recordKind() Kind  { return KindExternalOp }
func (*Progress) recordKind() Kind    { return KindProgress }
func (*Judgment) recordKind() Kind    { return KindJudgment }
func (*StageRun) recordKind() Kind    { return KindStageRun }
func (*Round) recordKind() Kind       { return KindRound }
func (*Instruction) recordKind() Kind { return KindInstruction }
func (*Artifact) recordKind() Kind    { return KindArtifact }

func (*ResultRejection) recordKind() Kind { return KindResultRejection }
func (*UnitResult) recordKind() Kind      { return KindUnitResult }
func (*Merge) recordKind() Kind           { return KindMerge }
func (*JudgmentBrief) recordKind() Kind   { return KindJudgmentBrief }
