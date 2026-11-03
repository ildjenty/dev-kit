package form

import "encoding/json"

// TerminationKind は作業者が報告する終了の種別を表す。
type TerminationKind string

const (
	TerminationCompleted TerminationKind = "完了を主張"
	TerminationQuestion  TerminationKind = "停止して確認"
	TerminationStopped   TerminationKind = "作業停止の申告"
)

// Progress は submit に渡す終了ブロックを表す。
type Progress struct {
	InstructionID    string           `json:"instruction_id"`
	TerminationKind  TerminationKind  `json:"termination_kind"`
	WorkPerformed    []string         `json:"work_performed,omitempty"`
	Confirmed        []string         `json:"confirmed,omitempty"`
	Unconfirmed      []string         `json:"unconfirmed,omitempty"`
	Obstacles        []string         `json:"obstacles,omitempty"`
	TemporaryState   []TemporaryState `json:"temporary_state,omitempty"`
	ResumeConditions []string         `json:"resume_conditions,omitempty"`
	OutOfScope       []string         `json:"out_of_scope,omitempty"`
	ChangeReason     string           `json:"change_reason,omitempty"`
}

// TemporaryState は再開時に有効性を確認する一時状態を表す。
type TemporaryState struct {
	Content            string `json:"content"`
	VerificationMethod string `json:"verification_method"`
	Source             string `json:"source"`
	InvalidWhen        string `json:"invalid_when"`
}

// Reference は作業内の記録か、明示した外部の出典を参照する。
type Reference struct {
	RecordID string `json:"record_id,omitempty"`
	External string `json:"external,omitempty"`
}

// TargetKind は判定対象の種別を表す。
type TargetKind string

const (
	TargetTree      TargetKind = "tree"
	TargetPlan      TargetKind = "plan"
	TargetStructure TargetKind = "structure"
)

// TargetVersion は Kind に対応する項目で対象を識別する。
type TargetVersion struct {
	Kind      TargetKind `json:"kind"`
	Tree      string     `json:"tree,omitempty"`
	Plan      int        `json:"plan,omitempty"`
	Structure string     `json:"structure,omitempty"`
}

// NormReference は規範のパスと、その tree ID または blob ID を表す。
type NormReference struct {
	Path   string `json:"path"`
	TreeID string `json:"tree_id,omitempty"`
	BlobID string `json:"blob_id,omitempty"`
}

// JudgmentValue は形式上受け付ける判定値を表す。
type JudgmentValue string

const (
	Pass        JudgmentValue = "PASS"
	Fail        JudgmentValue = "FAIL"
	NotAssessed JudgmentValue = "NOT ASSESSED"
)

// FindingKind は指摘の解決に人の判断が必要かを表す。
type FindingKind string

const (
	AutoFixable   FindingKind = "auto-fixable"
	NeedsDecision FindingKind = "needs-decision"
)

// Exemption は適用除外の主張と根拠を表す。適用の判断は check が担う。
type Exemption struct {
	Claim    string      `json:"claim"`
	Evidence []Reference `json:"evidence"`
}

// Finding は判定者が返す指摘の形式を表す。
type Finding struct {
	ID                 string      `json:"id"`
	Blocking           bool        `json:"blocking"`
	Kind               FindingKind `json:"kind"`
	Scenario           string      `json:"scenario"`
	Exemption          *Exemption  `json:"exemption,omitempty"`
	ResolvedFindingIDs []string    `json:"resolved_finding_ids,omitempty"`
}

// JudgmentResult は判定者の最終メッセージの形式を表す。
// 受理・集約と、runner や観測した来歴の付加は利用側が担う。
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

// PlannerResult は計画者の最終メッセージを表す。Plan の検証は plan が担う。
type PlannerResult struct {
	InstructionID string          `json:"instruction_id"`
	Plan          json.RawMessage `json:"plan"`
}

// ExternalOperationTarget は外部操作の承認対象を識別する。
type ExternalOperationTarget struct {
	OperationID    string `json:"operation_id"`
	Kind           string `json:"kind"`
	Target         string `json:"target"`
	InputHash      string `json:"input_hash"`
	DefinitionHash string `json:"definition_hash"`
	ExecutableHash string `json:"executable_hash"`
}

// ApprovalTargetKind は問いの承認対象の種別を表す。
type ApprovalTargetKind string

const (
	ApprovalTree     ApprovalTargetKind = "tree"
	ApprovalPlan     ApprovalTargetKind = "plan"
	ApprovalExternal ApprovalTargetKind = "external"
	ApprovalDecision ApprovalTargetKind = "decision"
	ApprovalWork     ApprovalTargetKind = "work"
)

// ApprovalTarget は Kind に対応する項目で承認対象を識別する。
type ApprovalTarget struct {
	Kind       ApprovalTargetKind       `json:"kind"`
	Tree       string                   `json:"tree,omitempty"`
	Plan       int                      `json:"plan,omitempty"`
	External   *ExternalOperationTarget `json:"external,omitempty"`
	DecisionID string                   `json:"decision_id,omitempty"`
	WorkID     string                   `json:"work_id,omitempty"`
}

// Question は人に提示する問いを表す。ScreenID は画面のデザインの承認時に使う。
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

// InstructionKind は next が返す指示の種別を表す。
type InstructionKind string

const (
	InstructionWork          InstructionKind = "work"
	InstructionJudgmentWait  InstructionKind = "judgment_wait"
	InstructionJudgment      InstructionKind = "judgment"
	InstructionPlanCheck     InstructionKind = "plan_check"
	InstructionDetectionTest InstructionKind = "detection_test"
	InstructionPlan          InstructionKind = "plan"
	InstructionQuestion      InstructionKind = "question"
	InstructionExternal      InstructionKind = "external"
	InstructionCompleted     InstructionKind = "completed"
	InstructionAborted       InstructionKind = "aborted"
)

// Instruction は発行済みの指示を表す。ID の導出と出し直しは利用側が担う。
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

// RejectedAlternative は却下した案と、その理由・具体例を表す。
type RejectedAlternative struct {
	Proposal string `json:"proposal"`
	Reason   string `json:"reason"`
	Example  string `json:"example"`
}

// Decision は判断の内容と、有効性・置換・依存を解決するための情報を表す。
// 承認・却下・保留の状態は持たず、記録から利用側が導く。
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

// Evidence は出典と確認方法を添えた調査結果を表す。
type Evidence struct {
	ID                 string      `json:"id"`
	Content            string      `json:"content"`
	Sources            []Reference `json:"sources"`
	VerificationMethod string      `json:"verification_method"`
}
