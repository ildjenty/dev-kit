package form

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func schemaFor(t *testing.T, name string) *Schema {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "assets", "schemas", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	resources := make(map[string][]byte)
	for _, dependency := range []string{"finding", "question", "reference", "decision", "evidence", "plan", "plan-template"} {
		if dependency == name {
			continue
		}
		data, err := os.ReadFile(filepath.Join("..", "..", "assets", "schemas", dependency+".json"))
		if err != nil {
			t.Fatal(err)
		}
		resources["urn:devkit:form:"+dependency] = data
		if dependency == "plan" || dependency == "plan-template" {
			resources["https://devkit.invalid/schemas/v1/"+dependency+".json"] = data
		}
	}
	schema, err := CompileSchemaWithResources(source, resources)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestProgressExample(t *testing.T) {
	data := []byte(`{
		"instruction_id":"work-instruction",
		"termination_kind":"completion-claimed",
		"work_performed":["Changed the save button label from Register to Save"],
		"confirmed":["The save button displays Save and existing tests pass"],
		"unconfirmed":[],"obstacles":[],"temporary_state":[],"resume_conditions":[]
	}`)
	var progress Progress
	if err := schemaFor(t, "progress").Decode(data, &progress); err != nil {
		t.Fatal(err)
	}
	if progress.InstructionID != "work-instruction" || progress.TerminationKind != TerminationCompleted {
		t.Fatalf("Parsed termination block: %+v", progress)
	}
	for _, kind := range []TerminationKind{TerminationCompleted, TerminationQuestion, TerminationStopped} {
		progress := Progress{InstructionID: "work-instruction", TerminationKind: kind}
		if kind == TerminationQuestion {
			progress.Confirmation = &Confirmation{Text: "Which scope should be used?", Options: []string{}, Evidence: []Reference{}}
		}
		data, err := json.Marshal(progress)
		if err != nil {
			t.Fatal(err)
		}
		if err := schemaFor(t, "progress").Validate(data); err != nil {
			t.Fatalf("Valid termination block was rejected: %v", err)
		}
	}
}

func TestFixedFormats(t *testing.T) {
	refs := []Reference{{RecordID: "record"}, {External: "docs/design.md"}}
	finding := Finding{
		ID: "finding", Blocking: true, Kind: NeedsDecision, Scenario: "Required judgment criteria cannot be confirmed",
		Exemption: &Exemption{Claim: "Exemption conditions", Evidence: refs}, ResolvedFindingIDs: []string{"previous"},
	}
	question := Question{
		ID: "question", Text: "Proceed with the screen design?", ScreenID: "settings",
		ApprovalTarget: ApprovalTarget{Kind: ApprovalTree, Tree: "tree"},
		Options:        []string{"Proceed", "Reconsider"}, Recommendation: "Proceed", RecommendationReason: "The displayed content has been checked",
		StopCategory: "design", NextStage: "design", StoppedStage: "requirements", StopReason: "Awaiting review of the displayed content", Evidence: refs,
		StopObservation: &Reference{External: "stops/observation.json"},
	}
	decisionQuestion := question
	decisionQuestion.ApprovalTarget = ApprovalTarget{Kind: ApprovalDecision, DecisionID: "decision"}
	decisionQuestion.Options = []string{"approve", "reject", "defer"}
	decisionQuestion.Recommendation = "approve"
	external := ExternalOperationTarget{
		OperationID: "operation", Kind: "publish", Target: "pull request",
		InputHash: "input", DefinitionHash: "definition", ExecutableHash: "executable",
	}
	externalQuestion := question
	externalQuestion.ApprovalTarget = ApprovalTarget{Kind: ApprovalExternal, External: &external}
	cases := []struct {
		name  string
		value any
	}{
		{"progress", Progress{
			InstructionID: "instruction", TerminationKind: TerminationStopped,
			TemporaryState: []TemporaryState{{Content: "Runtime environment", VerificationMethod: "Confirm startup", Source: "Execution log", InvalidWhen: "When the process exits"}},
		}},
		{"finding", finding},
		{"judgment", JudgmentResult{
			InstructionID: "instruction", Aspect: "wording", TargetVersion: TargetVersion{Kind: TargetTree, Tree: "tree"},
			Norms: []NormReference{{Path: "AGENTS.md", BlobID: "blob"}}, CompletionConditionIDs: []string{"save-label"},
			Judgment: NotAssessed, Findings: []Finding{finding}, CheckedScope: []string{}, UncheckedScope: []string{"Screen"},
			Method: "Open the screen", Environment: "Local",
		}},
		{"planner-result", PlannerResult{InstructionID: "instruction", Plan: json.RawMessage(`{"version":1,"pattern":{"name":"test","content_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"task":"Implement the task","completion_conditions":[{"id":"C1","description":"The task is complete"}],"start_stage":"implement","stages":[{"id":"implement","assignee":"worker","checks":[],"gate":{"required":false,"reason":"No gate required"},"limits":{"max_rounds":3,"max_no_progress":2},"transitions":{"fix":"implement","ask":"implement","back":"implement","done":null}}],"external_operations":[]}`)}},
		{"question", question},
		{"question", decisionQuestion},
		{"question", externalQuestion},
		{"instruction", Instruction{ID: "instruction", Kind: InstructionQuestion, TargetKey: "question", Question: &question}},
		{"instruction", Instruction{ID: "instruction", Kind: InstructionExternal, TargetKey: "operation", External: &external}},
		{"decision", Decision{
			ID: "decision", Content: "Use the existing save operation", Validity: "While the save destination remains unchanged", Supersedes: []Reference{}, DependsOn: refs,
			Conditions: []string{"Keep the existing save destination"}, Decider: "Human", Provisional: true,
			RejectedAlternatives: []RejectedAlternative{{Proposal: "Add a save operation", Reason: "Duplicates existing behavior", Example: "Saving the same input"}}, Evidence: refs,
		}},
		{"evidence", Evidence{ID: "evidence", Content: "The existing operation can save the data", Sources: refs, VerificationMethod: "Run existing tests"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if err := schemaFor(t, tc.name).Validate(data); err != nil {
				t.Fatal(err)
			}
			if text, err := RenderMarkdown(tc.value); err != nil || text == "" {
				t.Fatalf("Markdown rendering: %q, %v", text, err)
			}
		})
	}
	text, err := RenderMarkdown(question)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Screen ID: settings", "requirements", "Awaiting review of the displayed content", "your own words", "answer outside the options", "conditional answer", "ask a question in return", "revise your answer", "docs/design.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("Question output does not contain %q", want)
		}
	}
}

func TestInvalidFormats(t *testing.T) {
	cases := []struct{ schema, data string }{
		{"progress", `{"termination_kind":"completion-claimed"}`},
		{"progress", `{"instruction_id":"instruction"}`},
		{"progress", `{"instruction_id":"instruction","termination_kind":"PASS"}`},
		{"progress", `{"instruction_id":"instruction","termination_kind":"confirmation-required","temporary_state":[{"content":"State"}]}`},
		{"planner-result", `{"plan":{}}`},
		{"planner-result", `{"instruction_id":"instruction","plan":[]}`},
		{"finding", `{"id":"finding","blocking":true,"kind":"unknown","scenario":"Failure"}`},
		{"instruction", `{"id":"instruction","kind":"judgment","target_key":"round/aspect","reissue":0,"brief":"Request"}`},
		{"instruction", `{"id":"instruction","kind":"question","target_key":"question","reissue":0}`},
		{"instruction", `{"id":"instruction","kind":"external","target_key":"operation","reissue":0,"external":{"operation_id":"operation","kind":"publish","target":"pull request","input_hash":"input","definition_hash":"definition"}}`},
		{"evidence", `{"id":"evidence","content":"Result","verification_method":"Check"}`},
		{"evidence", `{"id":"evidence","content":"Result","verification_method":"Check","sources":[{"record_id":"record","external":"source"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.schema+"/"+tc.data, func(t *testing.T) {
			if err := schemaFor(t, tc.schema).Validate([]byte(tc.data)); err == nil {
				t.Fatal("Accepted an invalid format")
			}
		})
	}
	schema := schemaFor(t, "progress")
	if err := schema.Validate([]byte(`{"instruction_id":"i","termination_kind":"completion-claimed"} {}`)); err == nil {
		t.Fatal("Accepted multiple JSON values")
	}
}

func TestSchemaDoesNotLoadFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "external.json")
	if err := os.WriteFile(path, []byte(`{"type":"string"}`), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	source, err := json.Marshal(map[string]string{"$ref": uri})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompileSchema(source); err == nil {
		t.Fatal("Accepted a reference that was not supplied")
	}
	resource, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := CompileSchemaWithResources(source, map[string][]byte{uri: resource})
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`"supplied"`)); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`42`)); err == nil {
		t.Fatal("Ignored a supplied reference")
	}
}

func TestJudgmentBoundary(t *testing.T) {
	schema := schemaFor(t, "judgment")
	base := `{
		"instruction_id":"instruction","aspect":"wording",
		"target_version":{"kind":"plan","plan":1},
		"norms":[],"completion_condition_ids":[],"judgment":"PASS",
		"findings":[],"checked_scope":[],"unchecked_scope":[],
		"method":"Compare with the plan","environment":"Local"
	}`
	for _, field := range []string{"instruction_id", "judgment", "target_version", "findings"} {
		t.Run(field, func(t *testing.T) {
			var value map[string]json.RawMessage
			if err := json.Unmarshal([]byte(base), &value); err != nil {
				t.Fatal(err)
			}
			switch field {
			case "instruction_id":
				delete(value, field)
			case "judgment":
				value[field] = json.RawMessage(`"UNKNOWN"`)
			case "target_version":
				value[field] = json.RawMessage(`{"kind":"plan","plan":1,"tree":"tree"}`)
			case "findings":
				value[field] = json.RawMessage(`[{"id":"finding","blocking":true,"kind":"unknown","scenario":"Failure"}]`)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(data); err == nil {
				t.Fatal("Accepted an invalid judgment result")
			}
		})
	}
	if err := schema.Validate([]byte(base)); err != nil {
		t.Fatal(err)
	}
}

func TestTemplates(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  []string
	}{
		{"skill", SkillTemplate{Name: "devkit", Description: "Advance development", Purpose: "Purpose", Steps: "Steps"}, []string{"name: \"devkit\"", "description: \"Advance development\"", "## Steps"}},
		{"pr", PRTemplate{PlanSummary: "Label change plan", Decisions: "Use the existing operation", Verification: "Open and check the save button"}, []string{"Label change plan", "Use the existing operation", "Open and check the save button"}},
		{"design", DesignTemplate{Title: "Save screen", Irreversible: "Reference to the plan list"}, []string{"# Save screen", "Reference to the plan list"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", "..", "assets", "templates", tc.name+".md.tmpl"))
			if err != nil {
				t.Fatal(err)
			}
			text, err := ApplyTemplate(string(source), tc.value)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("Template output does not contain %q", want)
				}
			}
		})
	}
	custom := "## Repository verification heading\n\n{{.Verification}}\n"
	text, err := ApplyTemplate(custom, PRTemplate{Verification: "Verification results"})
	if err != nil || text != "## Repository verification heading\n\nVerification results\n" {
		t.Fatalf("Applying an existing template: %q, %v", text, err)
	}
	if _, err := ApplyTemplate("{{.missing}}", map[string]string{}); err == nil {
		t.Fatal("Accepted missing template input")
	}
}
