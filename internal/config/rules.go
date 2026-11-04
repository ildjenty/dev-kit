package config

import "fmt"

// CheckDefinition fixes how a named aspect is evaluated.
type CheckDefinition struct {
	Description    string      `toml:"description"`
	Runner         string      `toml:"runner"`
	Criteria       []Selection `toml:"criteria"`
	Schemas        []string    `toml:"schemas"`
	History        string      `toml:"history"`
	BriefByPlanner bool        `toml:"brief_by_planner"`
}

// Definitions accepts one definition, a list, or the wildcard in TOML.
type Definitions []string

func (d *Definitions) UnmarshalTOML(value any) error {
	switch value := value.(type) {
	case string:
		*d = Definitions{value}
	case []any:
		*d = Definitions{}
		for _, item := range value {
			name, ok := item.(string)
			if !ok {
				return fmt.Errorf("definition must contain strings")
			}
			*d = append(*d, name)
		}
	default:
		return fmt.Errorf("definition must be a string or an array of strings")
	}
	return nil
}

// StageRule constrains each instance of the selected definitions.
type StageRule struct {
	Definition    Definitions `toml:"definition"`
	Checks        []string    `toml:"checks"`
	Gate          bool        `toml:"gate"`
	Roles         []string    `toml:"roles"`
	MaxRounds     *int        `toml:"max_rounds"`
	MaxNoProgress *int        `toml:"max_no_progress"`
	Report        *Selection  `toml:"report"`
	History       string      `toml:"history"`
	Outputs       *[]string   `toml:"outputs"`
}

// PathRule constrains a work path; Definition is the source of Follows.
type PathRule struct {
	Require    Definitions `toml:"require"`
	Definition string      `toml:"definition"`
	Follows    string      `toml:"follows"`
	First      string      `toml:"first"`
	Last       string      `toml:"last"`
}

// LoopRules contains the configured structural constraints.
type LoopRules struct {
	Stage []StageRule `toml:"stage"`
	Path  []PathRule  `toml:"path"`
}
