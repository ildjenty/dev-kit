package plan

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

const (
	TemplateSchemaPath = "schemas/plan-template.json"
	PlanSchemaPath     = "schemas/plan.json"
	PlanSchemaVersion  = 1
)

type Assignee string

const (
	Worker Assignee = "worker"
	Human  Assignee = "human"
)

type Transition string

const (
	Fix  Transition = "fix"
	Ask  Transition = "ask"
	Back Transition = "back"
	Done Transition = "done"
)

type Transitions struct {
	Fix      string    `json:"fix"`
	FixRules []FixRule `json:"-"`
	Ask      string    `json:"ask"`
	Back     string    `json:"back"`
	Done     *string   `json:"done"`
}

func (t Transitions) Next(action Transition) (*string, error) {
	switch action {
	case Fix:
		return &t.Fix, nil
	case Ask:
		return &t.Ask, nil
	case Back:
		return &t.Back, nil
	case Done:
		return t.Done, nil
	default:
		return nil, fmt.Errorf("unknown transition: %q", action)
	}
}

type Limits struct {
	MaxRounds     int `json:"max_rounds"`
	MaxNoProgress int `json:"max_no_progress"`
}

// Reached takes the completed round count and the consecutive rounds without progress.
func (l Limits) Reached(rounds, noProgress int) bool {
	return rounds >= l.MaxRounds || noProgress >= l.MaxNoProgress
}

type Gate struct {
	Required bool   `json:"required"`
	Reason   string `json:"reason"`
}

type Runner string

func (r Runner) IsCommand() bool { return strings.HasPrefix(string(r), "command:") }

// Selection names a knowledge source and an adapter-specific selector.
type Selection struct {
	Source string `json:"source"`
	Select string `json:"select"`
}

// History selects optional context from fixed work history indexes.
type History string

const (
	HistoryNone     History = "none"
	HistoryStageRun History = "stage_run"
	HistoryCheck    History = "check"
	HistoryAll      History = "all"
)

// DefaultInputs returns the default knowledge selections for each role.
func DefaultInputs() []Selection {
	return []Selection{{Source: "requirements", Select: ""}, {Source: "rules", Select: ""}, {Source: "structure", Select: ""}}
}

// Check selects a configured check and binds it to this task.
type Check struct {
	Aspect                 string   `json:"aspect"`
	Required               bool     `json:"required"`
	Target                 string   `json:"target,omitempty"`
	CompletionConditionIDs []string `json:"completion_condition_ids,omitempty"`
	Instructions           []string `json:"instructions,omitempty"`
}

// CheckDefinition is the fixed configuration for a named check.
type CheckDefinition struct {
	Runner         Runner      `json:"runner"`
	Criteria       []Selection `json:"criteria,omitempty"`
	Schemas        []string    `json:"schemas,omitempty"`
	History        History     `json:"history,omitempty"`
	BriefByPlanner bool        `json:"brief_by_planner,omitempty"`
}

type Stage struct {
	Report       *Selection  `json:"report,omitempty"`
	UnitReport   *Selection  `json:"unit_report,omitempty"`
	HumanPrompt  *Selection  `json:"human_prompt,omitempty"`
	Delegations  []Selection `json:"delegations,omitempty"`
	Outputs      []string    `json:"outputs"`
	History      History     `json:"history,omitempty"`
	Instructions []string    `json:"instructions,omitempty"`
	Inputs       []Selection `json:"inputs"`
	ID           string      `json:"id"`
	Definition   string      `json:"definition,omitempty"`
	Split        bool        `json:"split,omitempty"`
	Units        []Unit      `json:"units,omitempty"`
	Assignee     Assignee    `json:"assignee"`
	Checks       []Check     `json:"checks"`
	Gate         Gate        `json:"gate"`
	Limits       Limits      `json:"limits"`
	Transitions  Transitions `json:"transitions"`
}

// Unit describes one worker's share of a split stage.
type Unit struct {
	Instructions []string `json:"instructions,omitempty"`
	Outputs      []string `json:"outputs"`
	Role         string   `json:"role,omitempty"`
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Scope        string   `json:"scope"`
}

// WorkerHistory returns the selected optional history scope for stage work.
func (s Stage) WorkerHistory() History {
	if s.History == "" {
		return HistoryStageRun
	}
	return s.History
}

// JudgmentHistory returns the optional history scope; required evidence is always supplied.
func (c CheckDefinition) JudgmentHistory() History {
	if c.History == "" {
		return HistoryCheck
	}
	return c.History
}

func (s Stage) RequiredChecks() []Check {
	checks := make([]Check, 0)
	for _, check := range s.Checks {
		if check.Required {
			checks = append(checks, cloneCheck(check))
		}
	}
	return checks
}

type CompletionRule struct {
	Checks             []Check
	RequireCommit      bool
	RequireTargetMatch bool
}

func (s Stage) CompletionRule() CompletionRule {
	return CompletionRule{s.RequiredChecks(), true, true}
}

// PlanTemplate is a reference pattern, not a constraint on task plans.
type PlanTemplate struct {
	Name           string  `json:"name"`
	StartStage     string  `json:"start_stage"`
	Stages         []Stage `json:"stages"`
	OptionalStages []Stage `json:"optional_stages"`
}

// ContentHash hashes the original pattern bytes, including any trailing newline.
func ContentHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

type PatternRef struct {
	Name        string `json:"name"`
	ContentHash string `json:"content_hash"`
}

type CompletionCondition struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type ExternalOperation struct {
	Kind           string `json:"kind"`
	Target         string `json:"target"`
	InputHash      string `json:"input_hash"`
	DefinitionHash string `json:"definition_hash"`
	ExecutableHash string `json:"executable_hash"`
}

type Plan struct {
	Version               int                   `json:"version"`
	Pattern               PatternRef            `json:"pattern"`
	Task                  string                `json:"task"`
	OpenQuestions         []string              `json:"open_questions,omitempty"`
	CompletionConditions  []CompletionCondition `json:"completion_conditions"`
	StartStage            string                `json:"start_stage"`
	Stages                []Stage               `json:"stages"`
	IrreversibleDecisions []string              `json:"irreversible_decisions,omitempty"`
	ExternalOperations    []ExternalOperation   `json:"external_operations"`
}

func (p Plan) Stage(id string) (Stage, bool) {
	for _, stage := range p.Stages {
		if stage.ID == id {
			stage = cloneStage(stage)
			if stage.Transitions.FixRules != nil {
				stage.Transitions.Fix = stage.ID
			}
			return stage, true
		}
	}
	return Stage{}, false
}

func (p Plan) Next(stageID string, action Transition) (*string, error) {
	stage, ok := p.Stage(stageID)
	if !ok {
		return nil, fmt.Errorf("stage not found: %q", stageID)
	}
	return stage.Transitions.Next(action)
}

func (p Plan) Declares(operation ExternalOperation) bool {
	return slices.Contains(p.ExternalOperations, operation)
}

// WorkerInputs returns the stage selections, or the defaults when omitted.
func (s Stage) WorkerInputs() []Selection { return roleInputs(s.Inputs) }

func roleInputs(inputs []Selection) []Selection {
	if inputs == nil {
		return DefaultInputs()
	}
	return slices.Clone(inputs)
}

func cloneCheck(c Check) Check {
	c.Instructions = slices.Clone(c.Instructions)
	c.CompletionConditionIDs = slices.Clone(c.CompletionConditionIDs)
	return c
}

// DefinitionID returns the stage definition, defaulting to the instance ID.
func (s Stage) DefinitionID() string {
	if s.Definition == "" {
		return s.ID
	}
	return s.Definition
}

func cloneStage(s Stage) Stage {
	s.Report = cloneSelection(s.Report)
	s.UnitReport = cloneSelection(s.UnitReport)
	s.HumanPrompt = cloneSelection(s.HumanPrompt)
	s.Delegations = slices.Clone(s.Delegations)
	s.Outputs = slices.Clone(s.Outputs)
	s.Instructions = slices.Clone(s.Instructions)
	s.Transitions.FixRules = slices.Clone(s.Transitions.FixRules)
	for i, rule := range s.Transitions.FixRules {
		if rule.When != nil {
			condition := *rule.When
			if condition.HumanRejected != nil {
				value := *condition.HumanRejected
				condition.HumanRejected = &value
			}
			s.Transitions.FixRules[i].When = &condition
		}
	}
	s.Inputs = slices.Clone(s.Inputs)
	s.Units = slices.Clone(s.Units)
	for i := range s.Units {
		s.Units[i].Instructions = slices.Clone(s.Units[i].Instructions)
		s.Units[i].Outputs = slices.Clone(s.Units[i].Outputs)
	}
	s.Checks = slices.Clone(s.Checks)
	for i := range s.Checks {
		s.Checks[i] = cloneCheck(s.Checks[i])
	}
	if s.Transitions.Done != nil {
		done := *s.Transitions.Done
		s.Transitions.Done = &done
	}
	return s
}

func cloneSelection(selection *Selection) *Selection {
	if selection == nil {
		return nil
	}
	copy := *selection
	return &copy
}

func clonePlan(p Plan) Plan {
	p.CompletionConditions = slices.Clone(p.CompletionConditions)
	p.OpenQuestions = slices.Clone(p.OpenQuestions)
	p.IrreversibleDecisions = slices.Clone(p.IrreversibleDecisions)
	p.ExternalOperations = slices.Clone(p.ExternalOperations)
	p.Stages = slices.Clone(p.Stages)
	for i := range p.Stages {
		p.Stages[i] = cloneStage(p.Stages[i])
	}
	return p
}
