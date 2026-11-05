package form

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestReportAndConfirmation(t *testing.T) {
	question := "Keep the old format?\nA conditional answer is welcome."
	progress := Progress{
		InstructionID: "work-instruction", TerminationKind: TerminationQuestion,
		Report: json.RawMessage(`{"proposal":{"format":"old"},"anything":[1,true]}`),
		Confirmation: &Confirmation{
			Text: question, Options: []string{"Keep", "Revise"},
			Recommendation: "Keep", RecommendationReason: "Existing consumers require it",
			Evidence: []Reference{{External: "fixed/diff.patch"}, {RecordID: "approval"}},
		},
	}
	data, err := json.Marshal(progress)
	if err != nil {
		t.Fatal(err)
	}
	var got Progress
	if err := schemaFor(t, "progress").Decode(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Confirmation.Text != question || string(got.Report) != string(progress.Report) {
		t.Fatalf("report or question changed: %+v", got)
	}
	markdown, err := RenderMarkdown(got)
	if err != nil || !strings.Contains(markdown, question) || !strings.Contains(markdown, "fixed/diff.patch") || !strings.Contains(markdown, `"format": "old"`) {
		t.Fatalf("report or confirmation not rendered: %q, %v", markdown, err)
	}
}

func TestConfirmationContract(t *testing.T) {
	schema := schemaFor(t, "progress")
	valid := `{"instruction_id":"i","termination_kind":"confirmation-required","confirmation":{"text":"Proceed?","options":[],"recommendation":"","recommendation_reason":"","evidence":[]}}`
	if err := schema.Validate([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		`{"instruction_id":"i","termination_kind":"confirmation-required"}`,
		strings.Replace(valid, `"text":"Proceed?"`, `"text":""`, 1),
		strings.Replace(valid, `"options":[]`, `"options":null`, 1),
		strings.Replace(valid, `"recommendation":""`, `"recommendation":"Proceed"`, 1),
		strings.Replace(valid, `"evidence":[]`, `"evidence":[{"external":""}]`, 1),
		strings.Replace(valid, `"text":"Proceed?"`, `"text":"Proceed?","unexpected":true`, 1),
		`{"instruction_id":"i","termination_kind":"completion-claimed","report":[]}`,
		`{"instruction_id":"i","termination_kind":"work-stopped","report":null}`,
	} {
		if err := schema.Validate([]byte(data)); err == nil {
			t.Fatalf("accepted invalid progress: %s", data)
		}
	}
	for _, kind := range []string{"completion-claimed", "work-stopped"} {
		data := `{"instruction_id":"i","termination_kind":"` + kind + `","report":{"partial":true}}`
		if err := schema.Validate([]byte(data)); err != nil {
			t.Fatalf("fixed format imposed a stage report schema: %v", err)
		}
	}
}

func TestStageReportContracts(t *testing.T) {
	for _, tc := range []struct {
		stage string
		valid string
	}{
		{"purpose", `{"users":["CLI users"],"purpose":"Keep work reviewable","requirements":["Retain the public contract"],"completion_conditions":[{"id":"C1","description":"The public contract is preserved","grounds":"Existing consumers depend on it"}],"agreement":"The human approved the proposal"}`},
		{"scope", `{"included":["Progress format"],"excluded":["Engine behavior"],"externally_visible":["New report fields"],"irreversible_items":[]}`},
		{"architecture", `{"decisions":[{"decision":"Use Go for the CLI","options":["Go","Python"],"recommendation":"Go","reason":"Single static binary","classification":"human","decider":"human","question_id":"Q1","answer_id":"A1","cost_and_operational_effects":"No runtime to install"}],"technology_choices":["Go 1.24"],"structure_reference":"fixed/structure.json","modules":[{"id":"form","responsibility":"Validate formats","path":"internal/form"}],"dependencies":[{"from":"engine","to":"form"}],"existing_locations":[],"rejected_alternatives":[{"proposal":"Python","reason":"Requires a runtime"}],"unverified_premises":[],"recovery_methods":["Replace the module behind its interface"]}`},
		{"increments", `{"increments":[{"id":"1","purpose":"Walking skeleton","completion_condition_ids":["C1"],"scope":"CLI entry to storage","size_grounds":"One module and one command","verification_method":"Run the end-to-end script","completion_conditions":["The CLI starts and stores one record"]}]}`},
		{"design", `{"decisions":[{"decision":"Use Go for the CLI","options":["Go","Python"],"recommendation":"Go","reason":"Single static binary","classification":"delegated","decider":"worker","question_id":"","answer_id":"","cost_and_operational_effects":"No runtime to install"}],"design_reference":"docs/design.md","structure_reference":"fixed/structure.json","interfaces":["Progress.Report: json.RawMessage"],"data_formats":["Report is a JSON object"],"failure_types":["Schema mismatch rejects submission"],"irreversible_contracts":[],"unverified_matters":[]}`},
		{"verification", `{"completion_conditions":[{"id":"C1","scenario":"Store a record","steps":["Run devkit add"],"observed_result":"The record is listed"}],"unconfirmed":[]}`},
		{"implement", `{"completion_conditions":[{"id":"C1","tests":["TestReportAndConfirmation"],"verification_results":["go test ./internal/form passed"]}],"changes":["Added report and confirmation fields"]}`},
		{"retrospective", `{"problems":[{"problem":"A previous report was unavailable to the judge","should_have_caught":"The judgment input test","destination":"machine checks","change":"Added a regression test","no_change_reason":""}],"requirements_updates":[],"norm_assessment":"The relevant rules remain useful"}`},
		{"research", `{"observations":["Progress has no stage report"],"inferences":["A separate field can preserve it"],"verified_scope":["The progress type"],"unverified_scope":[],"grounds":["internal/form/types.go"]}`},
		{"prototype", `{"premises":[{"premise":"The schema can accept a free object","experiment":"Compile and validate the schema","result":"Nested objects were accepted","decision":"Use a JSON object"}],"evidence_references":["fixed/results.json"],"prototype_disposition":"Removed the temporary experiment"}`},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			body, err := os.ReadFile("../../assets/templates/reports/" + tc.stage + ".json")
			if err != nil {
				t.Fatal(err)
			}
			schema, err := CompileSchema(body)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate([]byte(tc.valid)); err != nil {
				t.Fatalf("valid report rejected: %v", err)
			}
		})
	}
}
