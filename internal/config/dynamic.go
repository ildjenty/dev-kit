package config

import (
	"fmt"
	"slices"
	"strings"
)

func effectiveInputs(inputs []Selection) []Selection {
	if inputs == nil {
		return []Selection{{Source: "requirements"}, {Source: "rules"}, {Source: "structure"}}
	}
	return slices.Clone(inputs)
}

// PlanningInputs returns effective planner inputs, preserving an explicitly empty list.
func (c Config) PlanningInputs() []Selection { return effectiveInputs(c.Planning.Inputs) }

// PlanningMaxRounds returns the effective plan judgment round limit.
func (c Config) PlanningMaxRounds() int {
	if c.Planning.MaxRounds == 0 {
		return 3
	}
	return c.Planning.MaxRounds
}

// StageLimitCeilings returns effective global stage limit ceilings.
func (c Config) StageLimitCeilings() (maxRounds, maxNoProgress int) {
	rounds, noProgress := c.Limits.MaxRoundsCeiling, c.Limits.MaxNoProgressCeiling
	if rounds == 0 {
		rounds = 3
	}
	if noProgress == 0 {
		noProgress = 2
	}
	return rounds, noProgress
}

func (c Config) validateDynamicSettings() error {
	if c.Planning.MaxRounds < 1 {
		return fmt.Errorf("planning.max_rounds must be at least 1")
	}
	if c.Limits.MaxRoundsCeiling < 1 || c.Limits.MaxNoProgressCeiling < 1 {
		return fmt.Errorf("limits stage ceilings must be at least 1")
	}
	for _, selection := range append(c.PlanningInputs(), c.PlanCheck.Inputs...) {
		if err := c.ValidateSelection(selection); err != nil {
			return err
		}
	}
	for name, check := range c.Checks {
		if err := c.validateCheckDefinition(name, check); err != nil {
			return fmt.Errorf("checks.%s: %w", name, err)
		}
	}
	for i, rule := range c.Loop.Stage {
		if err := c.validateStageRule(rule); err != nil {
			return fmt.Errorf("loop.stage[%d]: %w", i, err)
		}
	}
	for i, rule := range c.Loop.Path {
		if err := validatePathRule(rule); err != nil {
			return fmt.Errorf("loop.path[%d]: %w", i, err)
		}
	}
	return nil
}

func (c Config) validateCheckDefinition(name string, check CheckDefinition) error {
	if !validCheckName(name) {
		return fmt.Errorf("invalid check name")
	}
	runner := string(check.Runner)
	if strings.HasPrefix(name, "command:") {
		if runner != name {
			return fmt.Errorf("command aspect and runner must match")
		}
		if len(check.Criteria) > 0 || check.Examples != nil {
			return fmt.Errorf("command checks must not select criteria or examples")
		}
	} else {
		if strings.HasPrefix(check.Runner, "command:") {
			return fmt.Errorf("command aspect and runner must match")
		}
		if _, ok := c.Judges[runner]; !ok {
			role, ok := c.RoleSettings()[runner]
			if !ok || role.Kind != "judge" {
				return fmt.Errorf("runner must name a judge role or registered judgment command")
			}
		}
	}
	if err := c.ValidateCriteria(check.Criteria); err != nil {
		return err
	}
	if check.Examples != nil {
		if err := c.ValidateSelection(*check.Examples); err != nil {
			return err
		}
	}
	if !validHistory(check.History) {
		return fmt.Errorf("invalid history")
	}
	for _, schema := range check.Schemas {
		if !sourceName.MatchString(schema) {
			return fmt.Errorf("invalid schema name: %q", schema)
		}
	}
	return nil
}

func validCheckName(name string) bool {
	return sourceName.MatchString(strings.TrimPrefix(name, "command:"))
}

func validHistory(history string) bool {
	return history == "" || slices.Contains([]string{"none", "stage_run", "check", "all"}, history)
}

func validateDefinitions(definitions Definitions, wildcard bool) error {
	if len(definitions) == 0 {
		return fmt.Errorf("at least one definition is required")
	}
	for i, definition := range definitions {
		if definition == "*" && wildcard && len(definitions) == 1 {
			continue
		}
		if !sourceName.MatchString(definition) || definition == "replan" {
			return fmt.Errorf("invalid definition: %q", definition)
		}
		if slices.Contains(definitions[:i], definition) {
			return fmt.Errorf("duplicate definition: %q", definition)
		}
	}
	return nil
}

func (c Config) validateStageRule(rule StageRule) error {
	if err := validateDefinitions(rule.Definition, true); err != nil {
		return err
	}
	for _, name := range rule.Checks {
		if _, ok := c.Checks[name]; !ok {
			return fmt.Errorf("unknown required check: %q", name)
		}
	}
	for _, id := range rule.Roles {
		role, ok := c.RoleSettings()[id]
		if !ok || role.Kind != "work" && role.Kind != "human" {
			return fmt.Errorf("role must be a work or human role: %q", id)
		}
	}
	if rule.MaxRounds != nil && *rule.MaxRounds < 1 || rule.MaxNoProgress != nil && *rule.MaxNoProgress < 1 {
		return fmt.Errorf("stage ceilings must be positive when set")
	}
	if !validHistory(rule.History) {
		return fmt.Errorf("invalid history")
	}
	if rule.Report != nil {
		if err := c.ValidateCriteria([]Selection{*rule.Report}); err != nil {
			return err
		}
	}
	if rule.Outputs != nil {
		for _, id := range *rule.Outputs {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("outputs must contain nonempty IDs")
			}
		}
	}
	return nil
}

func validatePathRule(rule PathRule) error {
	count := 0
	if rule.Require != nil {
		count++
		if err := validateDefinitions(rule.Require, false); err != nil {
			return err
		}
	}
	if rule.Follows != "" || rule.Definition != "" {
		count++
		if err := validateDefinitions(Definitions{rule.Definition}, false); err != nil {
			return err
		}
		if err := validateDefinitions(Definitions{rule.Follows}, false); err != nil {
			return err
		}
	}
	for _, definition := range []string{rule.First, rule.Last} {
		if definition != "" {
			count++
			if err := validateDefinitions(Definitions{definition}, false); err != nil {
				return err
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("exactly one of require, follows, first, or last is required")
	}
	return nil
}
