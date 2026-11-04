package brief

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func requestTemplate(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile("../../assets/templates/brief/" + name + ".md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func TestSuppliedTemplates(t *testing.T) {
	builders := []struct {
		name  string
		build func(string) (Brief, error)
	}{
		{"work", func(source string) (Brief, error) {
			return BuildWork(WorkInput{InstructionID: "custom-id", RequestTemplate: source})
		}},
		{"judgment", func(source string) (Brief, error) {
			return BuildJudgment(JudgmentInput{InstructionID: "custom-id", RequestTemplate: source})
		}},
		{"planner", func(source string) (Brief, error) {
			return BuildPlanner(PlannerInput{InstructionID: "custom-id", RequestTemplate: source})
		}},
	}
	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			got, err := builder.build("Custom request: {{.InstructionID}}")
			if err != nil {
				t.Fatal(err)
			}
			if got != (Brief{InstructionID: "custom-id", Text: "Custom request: custom-id"}) {
				t.Fatalf("supplied template was not applied: %+v", got)
			}
			for _, source := range []string{"{{", "{{.UnknownField}}"} {
				got, err := builder.build(source)
				if err == nil || got != (Brief{}) {
					t.Errorf("invalid template %q returned %+v, %v", source, got, err)
				}
			}
		})
	}
}

func TestBuildWork(t *testing.T) {
	previous := "  Previous stage output\n\n```text\n{{verbatim}}\n```\nTrailing spaces  \n"
	command := "Verification failed\n  expected: A\n  actual: B\n"
	input := WorkInput{
		RequestTemplate: requestTemplate(t, "work"),
		InstructionID:   "work-round-1",
		Stage:           plan.Stage{ID: "implement", Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2}},
		StageDefinition: "Implement the agreed contract.",
		Plan: plan.Plan{
			Task:                  "Allow users to save content",
			CompletionConditions:  []plan.CompletionCondition{{ID: "C1", Description: "Saved content can be read"}},
			IrreversibleDecisions: []string{"Irreversible plan decision"},
		},
		Purpose:          "Save user records",
		AllowedChanges:   []string{"internal/save/"},
		ForbiddenChanges: []string{"Public API"},
		Verification:     []string{"C1: Read after saving and compare the content"},
		PreviousOutputs:  []StageOutput{{StageID: "contract", Content: previous}},
		Repository: RepositoryFacts{
			LayoutGuides:          []Document{{Path: "internal/save/README.md", Content: "Storage layout guide"}},
			Rules:                 Document{Path: ".devkit/rules.md", Content: "Existing rules"},
			IrreversibleDecisions: []string{"Irreversible rule decision"},
			AvailableChecks:       []plan.Check{{Aspect: "command:test", Runner: "command:test"}},
		},
		Norms:            []Document{{Path: "AGENTS.md", Content: "Fixed instructions"}},
		PreviousFindings: []form.Finding{{ID: "F1", Blocking: true, Kind: form.AutoFixable, Scenario: "Saving fails"}},
		CommandOutputs:   []CommandOutput{{Check: plan.Check{Aspect: "command:test", Runner: "command:test", Required: true}, ExitCode: 1, Output: command}},
		PreCommitOutput:  "pre-commit verification failed\nDetails\n",
		OutputSchema:     `{"type":"object","required":["instruction_id"]}`,
	}
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildWork(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildWork(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != again || got.InstructionID != input.InstructionID {
		t.Fatal("assembled different requests from identical inputs")
	}
	for _, want := range []string{
		previous, command, input.StageDefinition, input.Purpose, input.OutputSchema,
		"C1", "Saved content can be read", "internal/save/", "Public API", input.Verification[0],
		"3 rounds", "2 consecutive rounds without progress", "internal/save/README.md", "Storage layout guide",
		".devkit/rules.md", "Existing rules", "Irreversible rule decision", "Irreversible plan decision",
		"AGENTS.md", "Fixed instructions", "F1", "Saving fails", "command:test", "Exit code: 1",
		input.PreCommitOutput, `"instruction_id": "work-round-1"`, string(form.TerminationQuestion), string(form.TerminationStopped),
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("request does not contain %q", want)
		}
	}
	after, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("assembly modified the input")
	}
}

func TestBuildJudgment(t *testing.T) {
	input := JudgmentInput{
		RequestTemplate: requestTemplate(t, "judgment"),
		InstructionID:   "judge-round-1",
		Content:         "Content selected by check\n  Target: snap/tree\nPrevious finding ID: F1\n{{do_not_expand}}\n",
		OutputSchema:    `{"required":["instruction_id","judgment"]}`,
	}
	got, err := BuildJudgment(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildJudgment(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != again || got.InstructionID != input.InstructionID {
		t.Fatal("assembled different requests from identical inputs")
	}
	for _, want := range []string{input.Content, input.OutputSchema, `"judge-round-1"`, "JSON-only final message", "instruction_id"} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("request does not contain %q", want)
		}
	}
}

func TestBuildPlanner(t *testing.T) {
	current := plan.Plan{Version: 1, Task: "Previous plan task"}
	remaining := plan.Plan{Task: "Remaining task", Stages: []plan.Stage{{ID: "implement"}}}
	input := PlannerInput{
		RequestTemplate:   requestTemplate(t, "planner"),
		InstructionID:     "plan-2",
		Version:           2,
		Template:          plan.PlanTemplate{Name: "fix", StartStage: "implement", PlanLimits: &plan.PlanLimits{MaxRounds: 4}},
		TemplateHash:      "template-hash",
		Task:              "Fix the save failure",
		StageDefinitions:  []Document{{Path: "stages/implement.md", Content: "Implementation definition"}},
		Repository:        RepositoryFacts{Rules: Document{Path: ".devkit/rules.md", Content: "Repository rules"}},
		Norms:             []Document{{Path: "principles/harness.md", Content: "Loop principles"}},
		CurrentPlan:       &current,
		RemainingPlan:     &remaining,
		HumanInstructions: "  Preserve human revision instructions\n{{do_not_interpolate}}\n",
		PreviousFindings:  []form.Finding{{ID: "PF1", Scenario: "Missing completion conditions"}},
		PreviousOutputs:   []StageOutput{{StageID: "research", Content: "Research results\n  Details\n"}},
		OutputSchema:      `{"required":["instruction_id","plan"]}`,
	}
	got, err := BuildPlanner(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildPlanner(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) || got.InstructionID != input.InstructionID {
		t.Fatal("assembled different requests from identical inputs")
	}
	for _, want := range []string{
		input.Task, input.TemplateHash, `"max_rounds": 4`, "Previous plan task", "Remaining task", "version to 2",
		"stages/implement.md", "Implementation definition", "Repository rules", "principles/harness.md", "Loop principles",
		input.HumanInstructions, "PF1", "Missing completion conditions", input.PreviousOutputs[0].Content,
		input.OutputSchema, "instruction_id", `"plan-2"`, "JSON-only final message",
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("request does not contain %q", want)
		}
	}
	initial, err := BuildPlanner(PlannerInput{RequestTemplate: input.RequestTemplate, InstructionID: "plan-1", Version: 1, Template: input.Template, Task: input.Task})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"## Current plan", "## Remaining work", "## Human revision instructions", "## Plan check findings and finding IDs"} {
		if strings.Contains(initial.Text, unwanted) {
			t.Errorf("initial request contains %q", unwanted)
		}
	}
}
