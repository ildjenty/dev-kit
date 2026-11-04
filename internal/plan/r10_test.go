package plan

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestStageInputsAndRemovedPatternSettings(t *testing.T) {
	v := validator(t)
	pattern, _ := loadTemplate(t, v, "docs")
	pattern.Stages[0].Inputs = []Selection{}
	data, err := json.Marshal(pattern)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := v.ParseTemplate(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Stages[0].Inputs == nil || len(parsed.Stages[0].WorkerInputs()) != 0 {
		t.Fatal("lost explicit empty inputs")
	}
	if !slices.Equal((Stage{}).WorkerInputs(), DefaultInputs()) {
		t.Fatal("lost default inputs")
	}
	for _, field := range []string{"planner_inputs", "judge_inputs", "plan_limits"} {
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		doc[field] = nil
		body, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.ParseTemplate(body); err == nil {
			t.Fatalf("accepted removed pattern field: %s", field)
		}
	}
}
