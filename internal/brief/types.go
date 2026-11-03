package brief

import (
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Brief は呼び出し側から渡された指示 ID と、保存する依頼本文を表す。
type Brief struct {
	InstructionID string
	Text          string
}

// Document は呼び出し側が解決した資料の参照先と本文を表す。
type Document struct {
	Path    string
	Content string
}

// StageOutput は採用した前のステージの出力を、内容を変えずに渡す。
type StageOutput struct {
	StageID string
	Content string
}

// RepositoryFacts は配置ガイド、規則とその変更困難な決定、利用可能な check を表す。
// IrreversibleDecisions には rules.md の項目を渡し、計画の項目は Plan から取り込む。
type RepositoryFacts struct {
	LayoutGuides          []Document
	Rules                 Document
	IrreversibleDecisions []string
	AvailableChecks       []plan.Check
}

// CommandOutput は呼び出し側が実行した必須の command: の check の結果を表す。
type CommandOutput struct {
	Check    plan.Check
	ExitCode int
	Output   string
}

// WorkInput は作業依頼に含める既知の情報を表す。
// Norms と各資料は固定した参照元から読み出し、一時状態は無効なものを除いて渡す。
type WorkInput struct {
	InstructionID    string
	Stage            plan.Stage
	StageDefinition  string
	Plan             plan.Plan
	Purpose          string
	AllowedChanges   []string
	ForbiddenChanges []string
	Verification     []string
	PreviousOutputs  []StageOutput
	Repository       RepositoryFacts
	Norms            []Document
	PreviousFindings []form.Finding
	CommandOutputs   []CommandOutput
	PreCommitOutput  string
	OutputSchema     string
}

// JudgmentInput は check が決めた依頼内容と出力スキーマを受け取る。
// Content には対象と場所、規範、観点、終了ブロック、前回の指摘 ID などを含める。
// 観点「差分」の有効な承認・委任を含め、依頼内容の選定は check が担う。
type JudgmentInput struct {
	InstructionID string
	Content       string
	OutputSchema  string
}

// PlannerInput は初回の計画、再提案、残りの計画の作り直しに用いる情報を表す。
// Version と TemplateHash は呼び出し側が決め、brief は生成や照合を行わない。
type PlannerInput struct {
	InstructionID     string
	Version           int
	Template          plan.PlanTemplate
	TemplateHash      string
	Task              string
	StageDefinitions  []Document
	Repository        RepositoryFacts
	Norms             []Document
	CurrentPlan       *plan.Plan
	RemainingPlan     *plan.Plan
	HumanInstructions string
	PreviousFindings  []form.Finding
	PreviousOutputs   []StageOutput
	OutputSchema      string
}
