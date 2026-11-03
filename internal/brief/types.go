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
}

// StageOutput passes an adopted previous stage output without changing its content.
type StageOutput struct {
	StageID string
	Content string
}

// RepositoryFacts contains layout guides, rules and their irreversible decisions, and available checks.
// IrreversibleDecisions receives items from rules.md; plan items come from Plan.
type RepositoryFacts struct {
	LayoutGuides          []Document
	Rules                 Document
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
	// RequestTemplate is the request text resolved by the caller from the fixed kit copy.
	RequestTemplate  string
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

// JudgmentInput receives the request content and output schema selected by check.
// Content includes the target and its location, norms, aspects, progress block, and previous finding IDs.
// check selects the request content, including valid approvals and delegations for the difference aspect.
type JudgmentInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed kit copy.
	RequestTemplate string
	InstructionID   string
	Content         string
	OutputSchema    string
}

// PlannerInput contains information for initial planning, revised proposals, and rebuilding the remaining plan.
// The caller determines Version and TemplateHash; brief neither generates nor verifies them.
type PlannerInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed kit copy.
	RequestTemplate   string
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
