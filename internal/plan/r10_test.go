package plan

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestRoleSelectionSchemaAndExplicitEmptyInputs(t *testing.T) {
	v := validator(t)
	template, _ := loadTemplate(t, v, "docs")
	template.PlannerInputs = []Selection{}
	template.JudgeInputs = []Selection{{Source: "context", Select: "judgment.md"}}
	template.Stages[0].Inputs = []Selection{}
	data, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := v.ParseTemplate(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.PlannerInputs == nil || len(parsed.PlannerSelections()) != 0 || parsed.Stages[0].Inputs == nil || len(parsed.Stages[0].WorkerInputs()) != 0 || !slices.Equal(parsed.JudgeSelections(), template.JudgeInputs) {
		t.Fatal("role selection round trip lost an explicit override")
	}
	if !slices.Equal((PlanTemplate{}).PlannerSelections(), DefaultInputs()) || !slices.Equal((PlanTemplate{}).JudgeSelections(), DefaultInputs()) || !slices.Equal((Stage{}).WorkerInputs(), DefaultInputs()) {
		t.Fatal("omitted role inputs lost the default data")
	}
	for _, role := range []string{"planner_inputs", "judge_inputs"} {
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		document[role] = []map[string]string{{"source": "bad/name", "select": ""}}
		malformed, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.ParseTemplate(malformed); err == nil {
			t.Fatalf("%s accepted an invalid selection", role)
		}
	}
}
