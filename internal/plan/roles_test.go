package plan

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFixRulesSchemaAndSelection(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "docs")
	original := newTestPlan(template, hash, "Review documentation", []CompletionCondition{{ID: "C1", Description: "Correct documentation"}})
	bindChecks(&original)
	rejected := true
	p := clonePlan(original)
	p.Stages[0].Transitions.FixRules = []FixRule{
		{When: &FixCondition{Check: "diff", FailuresAtLeast: 2}, Assignee: "senior"},
		{When: &FixCondition{HumanRejected: &rejected}, Assignee: "editor"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := v.ParsePlan(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, parsed) {
		t.Fatal("fix rule round trip changed the plan")
	}
	for _, tc := range []struct {
		facts FixFacts
		role  Assignee
		rule  int
	}{
		{FixFacts{Checks: []string{"diff"}, Failures: 2}, "senior", 1},
		{FixFacts{Checks: []string{"wording"}, Failures: 2}, Worker, 0},
		{FixFacts{Checks: []string{"diff"}, Failures: 1, HumanRejected: true}, "editor", 2},
		{FixFacts{}, Worker, 0},
	} {
		role, rule := p.Stages[0].FixAssignee(tc.facts)
		if role != tc.role || rule != tc.rule {
			t.Fatalf("fix selection = %s %d; want %s %d", role, rule, tc.role, tc.rule)
		}
	}

}

func TestFixRuleSchemaRejectsOtherConditionsAndTargets(t *testing.T) {
	v := validator(t)
	template, _ := loadTemplate(t, v, "docs")
	data, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	stage := doc["stages"].([]any)[0].(map[string]any)
	transitions := stage["transitions"].(map[string]any)
	for _, rule := range []map[string]any{
		{"to": "implement", "assignee": "worker"},
		{"when": map[string]any{"termination": "completion-claimed"}, "assignee": "worker"},
		{"when": map[string]any{"finding_category": "security"}, "assignee": "worker"},
		{"when": map[string]any{"failures_at_least": 0}, "assignee": "worker"},
	} {
		transitions["fix"] = []any{rule}
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.ParseTemplate(data); err == nil {
			t.Fatalf("accepted invalid fix rule: %+v", rule)
		}
	}
}
