package plan

import (
	"fmt"
	"reflect"
	"slices"
)

// StageRule constrains every instance of the selected definitions.
type StageRule struct {
	Definition    []string
	Checks        []string
	Gate          bool
	Roles         []string
	MaxRounds     int
	MaxNoProgress int
	Report        *Selection
	History       History
	Outputs       *[]string
}

// PathRule constrains the adopted path. Definition is the source of Follows.
type PathRule struct {
	Require    []string
	Definition string
	Follows    string
	First      string
	Last       string
}

// LoopRules holds stage and path constraints fixed with repository norms.
type LoopRules struct {
	Stage []StageRule
	Path  []PathRule
}

// RuleViolation identifies a rule by its zero-based index and an instance when applicable.
type RuleViolation struct {
	Rule    int
	StageID string
	Message string
}

// ValidateStageRules checks fixed stage fields and global limit ceilings.
func ValidateStageRules(p Plan, rules []StageRule, ceiling Limits) []RuleViolation {
	var result []RuleViolation
	for _, stage := range p.Stages {
		if stage.Limits.MaxRounds > ceiling.MaxRounds || stage.Limits.MaxNoProgress > ceiling.MaxNoProgress {
			result = append(result, RuleViolation{-1, stage.ID, "stage limits exceed global ceilings"})
		}
		for i, rule := range rules {
			if !slices.Contains(rule.Definition, "*") && !slices.Contains(rule.Definition, stage.DefinitionID()) {
				continue
			}
			for _, message := range stageRuleViolations(stage, rule) {
				result = append(result, RuleViolation{i, stage.ID, message})
			}
		}
	}
	return result
}

func stageRuleViolations(stage Stage, rule StageRule) []string {
	var result []string
	for _, aspect := range rule.Checks {
		if !slices.ContainsFunc(stage.Checks, func(check Check) bool { return check.Aspect == aspect && check.Required }) {
			result = append(result, fmt.Sprintf("required check is missing: %q", aspect))
		}
	}
	if rule.Gate && !stage.Gate.Required {
		result = append(result, "a human gate is required")
	}
	roles := []Assignee{stage.Assignee}
	for _, unit := range stage.Units {
		roles = append(roles, Assignee(unit.WorkRole()))
	}
	for _, fix := range stage.Transitions.FixRules {
		roles = append(roles, fix.Assignee)
	}
	for _, role := range roles {
		if rule.Roles != nil && !slices.Contains(rule.Roles, string(role)) {
			result = append(result, fmt.Sprintf("role is not allowed: %q", role))
		}
	}
	if rule.MaxRounds > 0 && stage.Limits.MaxRounds > rule.MaxRounds {
		result = append(result, "max_rounds exceeds the stage ceiling")
	}
	if rule.MaxNoProgress > 0 && stage.Limits.MaxNoProgress > rule.MaxNoProgress {
		result = append(result, "max_no_progress exceeds the stage ceiling")
	}
	if rule.Report != nil && !reflect.DeepEqual(rule.Report, stage.Report) {
		result = append(result, "report differs from the fixed selection")
	}
	if rule.History != "" && stage.WorkerHistory() != rule.History {
		result = append(result, "history differs from the fixed selection")
	}
	if rule.Outputs != nil && !reflect.DeepEqual(*rule.Outputs, stage.Outputs) {
		result = append(result, "outputs differ from the fixed selection")
	}
	return result
}

// EvaluatePath evaluates ordered adopted completions followed by the plan's done path.
// Completed stages retain their bound definitions and transitions. Follows never crosses replan.
// Require and Last are checked only when the proposed path ends at null.
func EvaluatePath(completed []Stage, p Plan, rules []PathRule) []RuleViolation {
	stages := slices.Clone(completed)
	seen := map[string]bool{}
	id := &p.StartStage
	for id != nil && *id != "replan" {
		if seen[*id] {
			return []RuleViolation{{-1, *id, "done transitions contain a cycle"}}
		}
		seen[*id] = true
		stage, ok := p.Stage(*id)
		if !ok {
			return []RuleViolation{{-1, *id, "done transition target not found"}}
		}
		stages = append(stages, stage)
		id = stage.Transitions.Done
	}
	terminal := id == nil
	var result []RuleViolation
	for i, rule := range rules {
		for _, violation := range pathRuleViolations(stages, rule, terminal) {
			violation.Rule = i
			result = append(result, violation)
		}
	}
	return result
}

func pathRuleViolations(stages []Stage, rule PathRule, terminal bool) []RuleViolation {
	var result []RuleViolation
	if len(stages) == 0 {
		return result
	}
	if rule.First != "" && stages[0].DefinitionID() != rule.First {
		result = append(result, RuleViolation{StageID: stages[0].ID, Message: "first definition must be " + rule.First})
	}
	if terminal {
		for _, required := range rule.Require {
			if !slices.ContainsFunc(stages, func(stage Stage) bool { return stage.DefinitionID() == required }) {
				result = append(result, RuleViolation{Message: "required definition is missing: " + required})
			}
		}
		if rule.Last != "" && stages[len(stages)-1].DefinitionID() != rule.Last {
			result = append(result, RuleViolation{StageID: stages[len(stages)-1].ID, Message: "last definition must be " + rule.Last})
		}
	}
	if rule.Follows == "" {
		return result
	}
	for index, stage := range stages {
		if stage.DefinitionID() == rule.Definition && !hasFollowing(stages, index, rule.Follows) {
			result = append(result, RuleViolation{StageID: stage.ID, Message: "definition must be followed in the same segment by " + rule.Follows})
		}
	}
	return result
}

func hasFollowing(stages []Stage, index int, definition string) bool {
	for i := index; i < len(stages); i++ {
		if i > index && stages[i].DefinitionID() == definition {
			return true
		}
		if stages[i].Transitions.Done == nil || *stages[i].Transitions.Done == "replan" {
			return false
		}
	}
	return false
}
