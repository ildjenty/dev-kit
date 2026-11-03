package check

import (
	"encoding/json"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
)

func TestFindingIDsAreLocalToAspect(t *testing.T) {
	request := judgmentRequest(t)
	schema, err := form.CompileSchemaWithResources(request.OutputSchema, map[string][]byte{
		"urn:devkit:form:finding":   asset(t, "kit/schemas/finding.json"),
		"urn:devkit:form:reference": asset(t, "kit/schemas/reference.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	accept := func(aspect string, findings []Finding, previous []Finding) Judgment {
		t.Helper()
		r := request
		r.Aspect.ID, r.InstructionID = aspect, aspect+"-instruction"
		r.PreviousFindings = FindingsForAspect(aspect, previous)
		r, err = PrepareRequest(r.RequestInput)
		if err != nil {
			t.Fatal(err)
		}
		result := form.JudgmentResult{
			InstructionID: r.InstructionID, Aspect: aspect, TargetVersion: r.TargetVersion,
			Norms: r.normReferences(), CompletionConditionIDs: []string{}, Judgment: Fail,
			Findings: findings, CheckedScope: []string{"Requested scope"}, UncheckedScope: []string{},
			Method: "Comparison with fixed norms", Environment: "Test fixture",
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		j, _, err := AcceptResult(r, data, schema, Provenance{Source: SubagentResult, InstructionID: r.InstructionID, Runner: r.Runner}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	finding := Finding{ID: "F1", Blocking: true, Kind: AutoFixable, Scenario: "The target violates a required condition"}
	first := accept("purpose", []Finding{finding}, nil)
	second := accept("diff", []Finding{finding}, nil)
	summary := Aggregate([]Judgment{first, second})
	if summary.Findings[0].ID == summary.Findings[1].ID {
		t.Fatal("different aspects shared a finding identity")
	}
	previous := FindingsForAspect("purpose", summary.Findings)
	if len(previous) != 1 || previous[0].ID != "F1" {
		t.Fatalf("correction request crossed aspect namespaces: %+v", previous)
	}
	resolution := finding
	resolution.ID, resolution.ResolvedFindingIDs = "F2", []string{"F1"}
	corrected := accept("purpose", []Finding{resolution}, summary.Findings)
	resolved := corrected.Findings[0].ResolvedFindingIDs[0]
	if resolved != first.Findings[0].ID || resolved == second.Findings[0].ID {
		t.Fatal("resolution crossed aspect namespaces")
	}
	if FindingID("a", "bc") == FindingID("ab", "c") {
		t.Fatal("finding identity encoding is ambiguous")
	}
}
