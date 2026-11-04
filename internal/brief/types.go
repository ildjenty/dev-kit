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
	ContentHash  string
	Path         string
	SnapshotPath string
}

// StageOutput passes an adopted previous stage output without changing its content.
// StageID is the instance ID; Definition is its stage definition ID.
type StageOutput struct {
	StageID    string
	Definition string
	Content    string
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
	Aspect   string
	Runner   string
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
// Criteria holds the fixed documents selected by the check's criteria.
type JudgmentInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate   string
	InstructionID     string
	Content           string
	Criteria          []Document
	SupportingSchemas []Document
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
// The caller determines Version and PatternHash; brief neither generates nor verifies them.
// Pattern is the prioritized pattern; other patterns and stage definitions are passed by path.
type PlannerInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate   string
	InstructionID     string
	Version           int
	Pattern           plan.PlanTemplate
	PatternHash       string
	Patterns          []NormPath
	StageCatalog      string
	StageDefinitions  []NormPath
	Roles             []RoleEntry
	LoopRules         LoopRules
	Checks            []CheckEntry
	Task              string
	OriginalRequest   string
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

// RoleEntry lists one configured role for the planner.
type RoleEntry struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Agent string `json:"agent,omitempty"`
}

// CheckEntry names one configured check with a one-line description.
type CheckEntry struct {
	Aspect      string `json:"aspect"`
	Description string `json:"description"`
}

// LoopRules lists the fixed loop rules and the global stage limit ceilings.
type LoopRules struct {
	Stage    []plan.StageRule `json:"stage"`
	Path     []plan.PathRule  `json:"path"`
	Ceilings plan.Limits      `json:"ceilings"`
}

// JudgmentBriefInput asks the planner how to check one round before a judgment. The result is
// supplementary and cannot narrow the fixed criteria or the judgment scope.
type JudgmentBriefInput struct {
	// RequestTemplate is the request text resolved by the caller from the fixed repository snapshot.
	RequestTemplate string
	InstructionID   string
	Check           plan.Check
	Runner          string
	Plan            plan.Plan
	Requirements    Document
	CurrentMaterial string
	OutputSchema    string
}
