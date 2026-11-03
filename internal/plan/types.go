package plan

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

const (
	TemplateSchemaPath = "kit/schemas/plan-template.json"
	PlanSchemaPath     = "kit/schemas/plan.json"
	PlanSchemaVersion  = 1
	DefaultPlanRounds  = 3
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
	Fix  string  `json:"fix"`
	Ask  string  `json:"ask"`
	Back string  `json:"back"`
	Done *string `json:"done"`
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
		return nil, fmt.Errorf("不明な遷移: %q", action)
	}
}

type Limits struct {
	MaxRounds     int `json:"max_rounds"`
	MaxNoProgress int `json:"max_no_progress"`
}

// Reached は完了済みのラウンド数と連続して進展がなかった回数を受け取る。
func (l Limits) Reached(rounds, noProgress int) bool {
	return rounds >= l.MaxRounds || noProgress >= l.MaxNoProgress
}

type PlanLimits struct {
	MaxRounds int `json:"max_rounds"`
}

type Gate struct {
	Required bool   `json:"required"`
	Reason   string `json:"reason"`
}

type Runner string

func (r Runner) IsCommand() bool { return strings.HasPrefix(string(r), "command:") }

// 判定者のサブエージェント名と判定コマンド名の解決は呼び出し側が行う。
type Check struct {
	Aspect                 string   `json:"aspect"`
	Runner                 Runner   `json:"runner"`
	Required               bool     `json:"required"`
	CompletionConditionIDs []string `json:"completion_condition_ids,omitempty"`
	Target                 string   `json:"target,omitempty"`
}

type Stage struct {
	ID          string      `json:"id"`
	Assignee    Assignee    `json:"assignee"`
	Checks      []Check     `json:"checks"`
	Gate        Gate        `json:"gate"`
	Limits      Limits      `json:"limits"`
	Transitions Transitions `json:"transitions"`
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

// CompletionRule の検出試験は、必須 check のうち判定者が担当する観点に適用する。
type CompletionRule struct {
	Checks                []Check
	RequireDetectionTests bool
	RequireCommit         bool
	RequireTargetMatch    bool
}

func (s Stage) CompletionRule() CompletionRule {
	return CompletionRule{s.RequiredChecks(), true, true, true}
}

type PlanTemplate struct {
	Name           string      `json:"name"`
	StartStage     string      `json:"start_stage"`
	MetaLoopStages []string    `json:"meta_loop_stages"`
	Stages         []Stage     `json:"stages"`
	OptionalStages []Stage     `json:"optional_stages"`
	PlanLimits     *PlanLimits `json:"plan_limits,omitempty"`
}

func (t PlanTemplate) PlanRoundsReached(rounds int) bool {
	limit := DefaultPlanRounds
	if t.PlanLimits != nil {
		limit = t.PlanLimits.MaxRounds
	}
	return rounds >= limit
}

// ContentHash は末尾の改行も含む雛形の元のバイト列から求める。
func ContentHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

type TemplateRef struct {
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
	Template              TemplateRef           `json:"template"`
	Task                  string                `json:"task"`
	CompletionConditions  []CompletionCondition `json:"completion_conditions"`
	StartStage            string                `json:"start_stage"`
	MetaLoopStages        []string              `json:"meta_loop_stages,omitempty"`
	Stages                []Stage               `json:"stages"`
	IrreversibleDecisions []string              `json:"irreversible_decisions,omitempty"`
	ExternalOperations    []ExternalOperation   `json:"external_operations"`
}

func (p Plan) Stage(id string) (Stage, bool) {
	for _, stage := range p.Stages {
		if stage.ID == id {
			return cloneStage(stage), true
		}
	}
	return Stage{}, false
}

func (p Plan) Next(stageID string, action Transition) (*string, error) {
	stage, ok := p.Stage(stageID)
	if !ok {
		return nil, fmt.Errorf("ステージがない: %q", stageID)
	}
	return stage.Transitions.Next(action)
}

func (p Plan) Declares(operation ExternalOperation) bool {
	return slices.Contains(p.ExternalOperations, operation)
}

func cloneCheck(c Check) Check {
	c.CompletionConditionIDs = slices.Clone(c.CompletionConditionIDs)
	return c
}

func cloneStage(s Stage) Stage {
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

func clonePlan(p Plan) Plan {
	p.CompletionConditions = slices.Clone(p.CompletionConditions)
	p.MetaLoopStages = slices.Clone(p.MetaLoopStages)
	p.IrreversibleDecisions = slices.Clone(p.IrreversibleDecisions)
	p.ExternalOperations = slices.Clone(p.ExternalOperations)
	p.Stages = slices.Clone(p.Stages)
	for i := range p.Stages {
		p.Stages[i] = cloneStage(p.Stages[i])
	}
	return p
}
