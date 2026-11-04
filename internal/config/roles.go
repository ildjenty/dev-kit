package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Role defines a repository role and its optional subagent.
type Role struct {
	Kind          string `toml:"kind"`
	Agent         string `toml:"agent"`
	MaxConcurrent *int   `toml:"max_concurrent"`
}

// Roles holds role definitions and the Claude read-only tool allowlist.
type Roles struct {
	ReadOnlyTools []string
	Definitions   map[string]Role
}

// UnmarshalTOML decodes the mixed roles table while rejecting unknown fields.
func (r *Roles) UnmarshalTOML(value any) error {
	table, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("roles must be a table")
	}
	r.Definitions = map[string]Role{}
	for id, value := range table {
		if id == "read_only_tools" {
			values, ok := value.([]any)
			if !ok {
				return fmt.Errorf("roles.read_only_tools must be an array of tool names")
			}
			r.ReadOnlyTools = []string{}
			for _, item := range values {
				tool, ok := item.(string)
				if !ok || strings.TrimSpace(tool) == "" {
					return fmt.Errorf("roles.read_only_tools must contain tool names")
				}
				r.ReadOnlyTools = append(r.ReadOnlyTools, tool)
			}
			continue
		}
		fields, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("roles.%s must be a table", id)
		}
		role, err := decodeRole(id, fields)
		if err != nil {
			return err
		}
		r.Definitions[id] = role
	}
	return nil
}

func decodeRole(id string, fields map[string]any) (Role, error) {
	role := Role{}
	for field, value := range fields {
		if field == "max_concurrent" {
			number, ok := value.(int64)
			if !ok || number < 1 || int64(int(number)) != number {
				return Role{}, fmt.Errorf("roles.%s.max_concurrent must be an integer of at least 1", id)
			}
			limit := int(number)
			role.MaxConcurrent = &limit
			continue
		}
		text, ok := value.(string)
		if !ok {
			return Role{}, fmt.Errorf("roles.%s.%s must be a string", id, field)
		}
		switch field {
		case "kind":
			role.Kind = text
		case "agent":
			role.Agent = text
		default:
			return Role{}, fmt.Errorf("unknown configuration key: roles.%s.%s", id, field)
		}
	}
	if len(fields) == 1 && role.MaxConcurrent != nil {
		if defaults, ok := (Config{}).RoleSettings()[id]; ok {
			defaults.MaxConcurrent = role.MaxConcurrent
			role = defaults
		}
	}
	return role, nil
}

// Planning selects the role that proposes plans.
type Planning struct {
	Role string `toml:"role"`
}

// RoleSettings returns effective repository roles.
func (c Config) RoleSettings() map[string]Role {
	roles := map[string]Role{
		"worker":  {Kind: "work", Agent: "devkit-worker"},
		"judge":   {Kind: "judge", Agent: "devkit-judge"},
		"planner": {Kind: "plan", Agent: "devkit-planner"},
		"human":   {Kind: "human"},
	}
	maps.Copy(roles, c.Roles.Definitions)
	return roles
}

// RoleMaxConcurrent returns a role's limit, which applies alongside the global limit.
func (c Config) RoleMaxConcurrent(id string) int {
	if limit := c.RoleSettings()[id].MaxConcurrent; limit != nil {
		return *limit
	}
	return c.MaxConcurrent()
}

// ReadOnlyTools returns tools permitted in Claude judge and planner definitions.
func (c Config) ReadOnlyTools() []string {
	if c.Roles.ReadOnlyTools == nil {
		return []string{"Read", "Glob", "Grep"}
	}
	return slices.Clone(c.Roles.ReadOnlyTools)
}

// PlanningRole returns the effective planning role ID.
func (c Config) PlanningRole() string {
	if c.Planning.Role == "" {
		return "planner"
	}
	return c.Planning.Role
}

// AgentRole resolves an agent's kind from effective role definitions.
func (c Config) AgentRole(agent string) (Role, bool) {
	if agent == "" {
		return Role{}, false
	}
	for _, role := range c.RoleSettings() {
		if role.Agent == agent {
			return role, true
		}
	}
	return Role{}, false
}

func (c Config) validateRoles() error {
	roles := c.RoleSettings()
	agents := map[string]string{}
	humans := 0
	for id, role := range roles {
		if !sourceName.MatchString(id) || id == "read_only_tools" {
			return fmt.Errorf("invalid role ID: %q", id)
		}
		if _, exists := c.Judges[id]; exists {
			return fmt.Errorf("role %q overlaps a registered judge name", id)
		}
		switch role.Kind {
		case "human":
			humans++
			if role.Agent != "" {
				return fmt.Errorf("roles.%s: human roles must not have an agent", id)
			}
		case "work", "judge", "plan":
			if role.Kind != "work" && role.Agent == "" {
				return fmt.Errorf("roles.%s.agent is required", id)
			}
		default:
			return fmt.Errorf("roles.%s.kind must be work, judge, plan, or human", id)
		}
		if role.Agent == "" {
			continue
		}
		if !strings.HasPrefix(role.Agent, "devkit-") || len(role.Agent) == len("devkit-") || !sourceName.MatchString(role.Agent) {
			return fmt.Errorf("roles.%s.agent must be a devkit- prefixed agent name", id)
		}
		if kind, exists := agents[role.Agent]; exists && kind != role.Kind {
			return fmt.Errorf("agent %q overlaps roles of different kinds", role.Agent)
		}
		agents[role.Agent] = role.Kind
	}
	if humans != 1 {
		return fmt.Errorf("roles must define exactly one human role")
	}
	planner, exists := roles[c.PlanningRole()]
	if !exists || planner.Kind != "plan" {
		return fmt.Errorf("planning.role must name a plan role")
	}
	runner := c.PlanCheckSettings().Runner
	if _, command := c.Judges[runner]; command {
		return nil
	}
	judge, exists := roles[runner]
	if !exists || judge.Kind != "judge" {
		return fmt.Errorf("plan_check.runner must name a judge role or registered judgment command")
	}
	if judge.Agent == planner.Agent {
		return fmt.Errorf("plan check and planning agents must differ")
	}
	return nil
}

func (c Config) validateRoleConcurrency() error {
	for id, role := range c.RoleSettings() {
		if role.MaxConcurrent != nil && *role.MaxConcurrent < 1 {
			return fmt.Errorf("roles.%s.max_concurrent must be at least 1", id)
		}
	}
	return nil
}
