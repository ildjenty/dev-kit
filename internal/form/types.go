package form

import "encoding/json"

// TerminationKind identifies the termination kind reported by the worker.
type TerminationKind string

const (
	TerminationCompleted TerminationKind = "completion-claimed"
	TerminationQuestion  TerminationKind = "confirmation-required"
	TerminationStopped   TerminationKind = "work-stopped"
)

// Progress is the termination block passed to submit.
type Progress struct {
	InstructionID    string           `json:"instruction_id"`
	TerminationKind  TerminationKind  `json:"termination_kind"`
	Report           json.RawMessage  `json:"report,omitempty"`
	Confirmation     *Confirmation    `json:"confirmation,omitempty"`
	CommitMessage    string           `json:"commit_message,omitempty"`
	WorkPerformed    []string         `json:"work_performed,omitempty"`
	Confirmed        []string         `json:"confirmed,omitempty"`
	Unconfirmed      []string         `json:"unconfirmed,omitempty"`
	Obstacles        []string         `json:"obstacles,omitempty"`
	TemporaryState   []TemporaryState `json:"temporary_state,omitempty"`
	ResumeConditions []string         `json:"resume_conditions,omitempty"`
	OutOfScope       []string         `json:"out_of_scope,omitempty"`
	ChangeReason     string           `json:"change_reason,omitempty"`
}

// Confirmation carries a worker's question verbatim, separately from its stage report.
type Confirmation struct {
	Text                 string      `json:"text"`
	Options              []string    `json:"options"`
	Recommendation       string      `json:"recommendation"`
	RecommendationReason string      `json:"recommendation_reason"`
	Evidence             []Reference `json:"evidence"`
}

// TemporaryState describes temporary state whose validity must be checked on resumption.
type TemporaryState struct {
	Content            string `json:"content"`
	VerificationMethod string `json:"verification_method"`
	Source             string `json:"source"`
	InvalidWhen        string `json:"invalid_when"`
}

// Reference points to a record within the work or an explicit external source.
type Reference struct {
	RecordID string `json:"record_id,omitempty"`
	External string `json:"external,omitempty"`
}

// TargetKind identifies the kind of judgment target.
type TargetKind string

const (
	TargetTree      TargetKind = "tree"
	TargetPlan      TargetKind = "plan"
	TargetStructure TargetKind = "structure"
)

// TargetVersion identifies the target using the field corresponding to Kind.
// For a tree, Sources hashes the set of source names, document IDs, and content hashes of
// fixed outside product documents.
type TargetVersion struct {
	Kind      TargetKind `json:"kind"`
	Tree      string     `json:"tree,omitempty"`
	Sources   string     `json:"sources,omitempty"`
	Plan      int        `json:"plan,omitempty"`
	Structure string     `json:"structure,omitempty"`
}

// NormReference contains a norm path and its tree ID or blob ID.
type NormReference struct {
	Path   string `json:"path"`
	TreeID string `json:"tree_id,omitempty"`
	BlobID string `json:"blob_id,omitempty"`
}

// JudgmentValue is a judgment value accepted by the format.
type JudgmentValue string

const (
	Pass        JudgmentValue = "PASS"
	Fail        JudgmentValue = "FAIL"
	NotAssessed JudgmentValue = "NOT ASSESSED"
)

// FindingKind indicates whether resolving a finding requires human judgment.
type FindingKind string

const (
	AutoFixable   FindingKind = "auto-fixable"
	NeedsDecision FindingKind = "needs-decision"
)

// Exemption contains an exemption claim and its evidence. The check package decides applicability.
type Exemption struct {
	Claim    string      `json:"claim"`
	Evidence []Reference `json:"evidence"`
}

// Finding is the format of a finding returned by a judge.
type Finding struct {
	PremiseBroken      bool        `json:"premise_broken,omitempty"`
	ID                 string      `json:"id"`
	Blocking           bool        `json:"blocking"`
	Kind               FindingKind `json:"kind"`
	Scenario           string      `json:"scenario"`
	Exemption          *Exemption  `json:"exemption,omitempty"`
	ResolvedFindingIDs []string    `json:"resolved_finding_ids,omitempty"`
}

// JudgmentResult is the format of a judge's final message.
// The caller handles acceptance, aggregation, and the addition of runner and observed provenance.
type JudgmentResult struct {
	InstructionID          string          `json:"instruction_id"`
	Aspect                 string          `json:"aspect"`
	TargetVersion          TargetVersion   `json:"target_version"`
	Norms                  []NormReference `json:"norms"`
	CompletionConditionIDs []string        `json:"completion_condition_ids"`
	Judgment               JudgmentValue   `json:"judgment"`
	Findings               []Finding       `json:"findings"`
	CheckedScope           []string        `json:"checked_scope"`
	UncheckedScope         []string        `json:"unchecked_scope"`
	Method                 string          `json:"method"`
	Environment            string          `json:"environment"`
}

// PlannerResult is the planner's final message. The plan package validates Plan.
type PlannerResult struct {
	InstructionID string          `json:"instruction_id"`
	Plan          json.RawMessage `json:"plan"`
}

// ExternalOperationTarget identifies the approval target of an external operation.
type ExternalOperationTarget struct {
	OperationID    string `json:"operation_id"`
	Kind           string `json:"kind"`
	Target         string `json:"target"`
	InputHash      string `json:"input_hash"`
	DefinitionHash string `json:"definition_hash"`
	ExecutableHash string `json:"executable_hash"`
}

// ApprovalTargetKind identifies the kind of approval target in a question.
type ApprovalTargetKind string

const (
	ApprovalTree     ApprovalTargetKind = "tree"
	ApprovalPlan     ApprovalTargetKind = "plan"
	ApprovalExternal ApprovalTargetKind = "external"
	ApprovalDecision ApprovalTargetKind = "decision"
	ApprovalWork     ApprovalTargetKind = "work"
)

// ApprovalTarget identifies the approval target using the field corresponding to Kind.
// A tree approval carries the same Sources identity as the approved target version.
// SourcePin identifies immutable outside norms when approving a norm snapshot.
type ApprovalTarget struct {
	Kind       ApprovalTargetKind       `json:"kind"`
	Tree       string                   `json:"tree,omitempty"`
	Sources    string                   `json:"sources,omitempty"`
	SourcePin  string                   `json:"source_pin,omitempty"`
	Plan       int                      `json:"plan,omitempty"`
	External   *ExternalOperationTarget `json:"external,omitempty"`
	DecisionID string                   `json:"decision_id,omitempty"`
	WorkID     string                   `json:"work_id,omitempty"`
}

// Question is a question presented to a person. ScreenID is used for screen design approval.
type Question struct {
	ID                   string         `json:"id"`
	Text                 string         `json:"text"`
	ApprovalTarget       ApprovalTarget `json:"approval_target"`
	Options              []string       `json:"options"`
	Recommendation       string         `json:"recommendation"`
	RecommendationReason string         `json:"recommendation_reason,omitempty"`
	StopCategory         string         `json:"stop_category"`
	NextStage            string         `json:"next_stage"`
	StoppedStage         string         `json:"stopped_stage"`
	StopReason           string         `json:"stop_reason"`
	Evidence             []Reference    `json:"evidence"`
	ScreenID             string         `json:"screen_id,omitempty"`
	StopObservation      *Reference     `json:"stop_observation,omitempty"`
}

// InstructionKind identifies the kind of instruction returned by next.
type InstructionKind string

const (
	InstructionWork          InstructionKind = "work"
	InstructionUnitWork      InstructionKind = "unit_work"
	InstructionJudgmentWait  InstructionKind = "judgment_wait"
	InstructionJudgment      InstructionKind = "judgment"
	InstructionPlanCheck     InstructionKind = "plan_check"
	InstructionPlan          InstructionKind = "plan"
	InstructionQuestion      InstructionKind = "question"
	InstructionExternal      InstructionKind = "external"
	InstructionCompleted     InstructionKind = "completed"
	InstructionAborted       InstructionKind = "aborted"
	InstructionJudgmentBrief InstructionKind = "judgment_brief"
)

// Instruction is an issued instruction. The caller handles ID derivation and reissuance.
type Instruction struct {
	ID         string                   `json:"id"`
	Kind       InstructionKind          `json:"kind"`
	TargetKey  string                   `json:"target_key"`
	Reissue    int                      `json:"reissue"`
	Text       string                   `json:"text,omitempty"`
	Brief      string                   `json:"brief,omitempty"`
	TargetPath string                   `json:"target_path,omitempty"`
	Runner     string                   `json:"runner,omitempty"`
	Question   *Question                `json:"question,omitempty"`
	External   *ExternalOperationTarget `json:"external,omitempty"`
}

// RejectedAlternative describes a rejected proposal, its reason, and a concrete example.
type RejectedAlternative struct {
	Proposal string `json:"proposal"`
	Reason   string `json:"reason"`
	Example  string `json:"example"`
}

// Decision contains a decision and information for resolving validity, supersession, and dependencies.
// It has no approval, rejection, or deferral state; the caller derives that state from records.
type Decision struct {
	ID                   string                `json:"id"`
	Content              string                `json:"content"`
	Validity             string                `json:"validity"`
	Supersedes           []Reference           `json:"supersedes"`
	DependsOn            []Reference           `json:"depends_on"`
	Conditions           []string              `json:"conditions"`
	Decider              string                `json:"decider"`
	Provisional          bool                  `json:"provisional"`
	RejectedAlternatives []RejectedAlternative `json:"rejected_alternatives"`
	Evidence             []Reference           `json:"evidence"`
}

// Evidence contains investigation results with sources and a verification method.
type Evidence struct {
	ID                 string      `json:"id"`
	Content            string      `json:"content"`
	Sources            []Reference `json:"sources"`
	VerificationMethod string      `json:"verification_method"`
}
