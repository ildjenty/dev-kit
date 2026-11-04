package brief

import (
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Brief contains the caller-supplied instruction ID and request text to be stored.
type Brief struct {
	InstructionID string
	Text          string
}

// Document contains a reference and document text resolved by the caller.
type Document struct {
	Path    string
	Content string
	Exists  bool
}

// NormPath maps a logical name to its fixed snapshot location.
type NormPath struct {
	Path         string
	SnapshotPath string
}

// StageOutput passes an adopted previous stage output without changing its content.
type StageOutput struct {
	StageID string
	Content string
}

// RepositoryFacts contains layout guides, rules and their irreversible decisions, and available checks.
// IrreversibleDecisions refers to shared rules; plan items come from Plan.
type RepositoryFacts struct {
	LayoutGuides          []Document
	Rules                 Document
	Requirements          Document
	Structure             Document
	IrreversibleDecisions []string
	AvailableChecks       []plan.Check
}

// CommandOutput contains the result of a required command: check executed by the caller.
type CommandOutput struct {
	Check    plan.Check
	ExitCode int
	Output   string
}

// WorkInput contains known information to include in a work request.
// Read Norms and documents from the fixed reference source and preserve conditions with their items.
type WorkInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate   string
	InstructionID     string
	Stage             plan.Stage
	StageDefinition   string
	Plan              plan.Plan
	Purpose           string
	Verification      []string
	PreviousOutputs   []StageOutput
	Repository        RepositoryFacts
	Norms             []Document
	NormPaths         []NormPath
	WorkHistoryPath   string
	SupportingSchemas []Document
	PreviousFindings  []form.Finding
	CommandOutputs    []CommandOutput
	PreCommitOutput   string
	OutputSchema      string
	Unit              *plan.Unit
	WorktreePath      string
	// CurrentMaterial holds this round's material: the previous round report and human answers.
	CurrentMaterial string
	// ReportSchema is the selected stage or unit report schema body.
	ReportSchema string
}

// JudgmentInput receives the request content and output schema selected by check.
// Content includes the target and its location, norms, progress block, and previous finding IDs.
// Criteria holds the fixed documents selected by the check's criteria. A detection request
// sets Detection and receives only criteria, anonymous examples in Content, and the output contract.
type JudgmentInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate   string
	InstructionID     string
	Content           string
	Criteria          []Document
	SupportingSchemas []Document
	Detection         bool
	Repository        RepositoryFacts
	Norms             []Document
	NormPaths         []NormPath
	OutputSchema      string
	// Instructions are the plan's check instructions; PreJudgmentInstructions are the planner's
	// supplementary instructions for this round. Neither narrows the criteria or scope.
	Instructions            []string
	PreJudgmentInstructions []string
	PreviousOutputs         []StageOutput
	CurrentMaterial         string
	WorkHistoryPath         string
	// Delegations are template-selected standing delegation documents, with Path as provenance.
	Delegations []Document
}

// PlannerInput contains information for initial planning, revised proposals, and rebuilding the remaining plan.
// The caller determines Version and TemplateHash; brief neither generates nor verifies them.
type PlannerInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate   string
	InstructionID     string
	Version           int
	Template          plan.PlanTemplate
	TemplateHash      string
	Task              string
	OriginalRequest   string
	StageDefinitions  []Document
	Repository        RepositoryFacts
	Norms             []Document
	NormPaths         []NormPath
	CurrentPlan       *plan.Plan
	RemainingPlan     *plan.Plan
	HumanInstructions string
	PreviousFindings  []form.Finding
	PreviousOutputs   []StageOutput
	OutputSchema      string
}

// JudgmentBriefInput asks the planner how to check one round before a judgment. The result is
// supplementary and cannot narrow the fixed criteria or the judgment scope.
type JudgmentBriefInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate string
	InstructionID   string
	Check           plan.Check
	Plan            plan.Plan
	Requirements    Document
	CurrentMaterial string
	OutputSchema    string
}
