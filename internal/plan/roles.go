package plan

import (
	"encoding/json"
	"slices"
)

// FixCondition matches recorded facts of the current stage run.
type FixCondition struct {
	Check           string `json:"check,omitempty"`
	FailuresAtLeast int    `json:"failures_at_least,omitempty"`
	HumanRejected   *bool  `json:"human_rejected,omitempty"`
}

// FixRule changes only the assignee of the next correction round.
type FixRule struct {
	When     *FixCondition `json:"when,omitempty"`
	Assignee Assignee      `json:"assignee"`
}

// FixFacts contains the recorded facts used by fix rules.
type FixFacts struct {
	Checks        []string
	Failures      int
	HumanRejected bool
}

// FixAssignee returns the first matching assignee and its one-based rule number.
func (s Stage) FixAssignee(facts FixFacts) (Assignee, int) {
	for i, rule := range s.Transitions.FixRules {
		if rule.When == nil || rule.When.matches(facts) {
			return rule.Assignee, i + 1
		}
	}
	return s.Assignee, 0
}

func (c FixCondition) matches(facts FixFacts) bool {
	return (c.Check == "" || slices.Contains(facts.Checks, c.Check)) && facts.Failures >= c.FailuresAtLeast && (c.HumanRejected == nil || *c.HumanRejected == facts.HumanRejected)
}

// MarshalJSON preserves the scalar fix form when no rules are present.
func (t Transitions) MarshalJSON() ([]byte, error) {
	var fix any = t.Fix
	if t.FixRules != nil {
		fix = t.FixRules
	}
	return json.Marshal(struct {
		Fix  any     `json:"fix"`
		Ask  string  `json:"ask"`
		Back string  `json:"back"`
		Done *string `json:"done"`
	}{fix, t.Ask, t.Back, t.Done})
}

// UnmarshalJSON reads scalar fix transitions or an ordered assignee rule list.
func (t *Transitions) UnmarshalJSON(data []byte) error {
	var raw struct {
		Fix  json.RawMessage `json:"fix"`
		Ask  string          `json:"ask"`
		Back string          `json:"back"`
		Done *string         `json:"done"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = Transitions{Ask: raw.Ask, Back: raw.Back, Done: raw.Done}
	if len(raw.Fix) > 0 && raw.Fix[0] == '[' {
		return json.Unmarshal(raw.Fix, &t.FixRules)
	}
	return json.Unmarshal(raw.Fix, &t.Fix)
}

// WorkRole returns the unit's role ID, defaulting to worker.
func (u Unit) WorkRole() string {
	if u.Role == "" {
		return "worker"
	}
	return u.Role
}
