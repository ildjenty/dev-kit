package check

import (
	"encoding/json"
	"io/fs"
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/assets"
	"github.com/ildjenty/dev-kit/internal/form"
)

func asset(t *testing.T, path string) []byte {
	t.Helper()
	data, err := fs.ReadFile(assets.FS(), path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestKitAspects(t *testing.T) {
	paths, err := fs.Glob(assets.FS(), "checks/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if path == "checks/README.md" {
			continue
		}
		aspect, err := ParseAspect(string(asset(t, path)))
		if err != nil || aspect.Blocking == "" || aspect.OutputFormat == "" {
			t.Fatalf("%s: %+v, %v", path, aspect, err)
		}
		if _, err := ParseDetectionSpec(asset(t, "examples/"+aspect.ID+"/spec.json")); err != nil {
			t.Fatalf("%s detection test: %v", aspect.ID, err)
		}
	}
	if _, err := ParseAspect("# incomplete\n- blocking: condition"); err == nil {
		t.Fatal("accepted an incomplete aspect")
	}
}

func TestAspectTemplateAndExistingKeys(t *testing.T) {
	want := Aspect{ID: "example", Checks: "checks", Blocking: "condition", DoNotReport: "outside scope", OutputFormat: "JSON"}
	rendered, err := form.ApplyTemplate(string(asset(t, "templates/aspect.md.tmpl")), want)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		rendered,
		"# example\n- \u78ba\u8a8d\u4e8b\u9805: checks\n- blocking: condition\n- \u5831\u544a\u3057\u306a\u3044\u4e8b\u9805: outside scope\n- \u51fa\u529b\u5f62\u5f0f: JSON",
	} {
		aspect, err := ParseAspect(source)
		if err != nil || aspect.ID != want.ID || aspect.Checks != want.Checks || aspect.Blocking != want.Blocking || aspect.DoNotReport != want.DoNotReport || aspect.OutputFormat == "" {
			t.Fatalf("parsed aspect: %+v, %v", aspect, err)
		}
	}
}

func judgmentRequest(t *testing.T) Request {
	t.Helper()
	request, err := PrepareRequest(RequestInput{
		Kind: RoundRequest, InstructionID: "judgment-1", Runner: "review",
		Aspect: Aspect{ID: "purpose"}, TargetVersion: form.TargetVersion{Kind: form.TargetTree, Tree: "tree"}, TargetPath: "snap/tree",
		Norms:        []Norm{{Reference: form.NormReference{Path: "AGENTS.md", BlobID: "blob"}}},
		OutputSchema: asset(t, "schemas/judgment.json"), CompletionConditionIDs: []string{"C1"},
		Progress:         &form.Progress{InstructionID: "work-1", TerminationKind: form.TerminationCompleted},
		PreviousFindings: []Finding{{ID: "old"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestAcceptResult(t *testing.T) {
	request := judgmentRequest(t)
	schema, err := form.CompileSchemaWithResources(request.OutputSchema, map[string][]byte{
		"urn:devkit:form:finding":   asset(t, "schemas/finding.json"),
		"urn:devkit:form:reference": asset(t, "schemas/reference.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := form.JudgmentResult{
		InstructionID: request.InstructionID, Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion,
		Norms: request.normReferences(), CompletionConditionIDs: []string{"C1"}, Judgment: Pass,
		Findings:     []Finding{{ID: "new", Kind: AutoFixable, Scenario: "out-of-scope reference information", ResolvedFindingIDs: []string{"old"}}},
		CheckedScope: []string{"purpose"}, UncheckedScope: []string{}, Method: "comparison against completion conditions", Environment: "example target",
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	provenance := Provenance{Source: SubagentResult, InstructionID: request.InstructionID, Runner: "review"}
	judgment, accepted, err := AcceptResult(request, data, schema, provenance, nil)
	if err != nil || !accepted || judgment.Judgment != Pass || !slices.Equal(judgment.Content, data) {
		t.Fatalf("could not accept judgment: %+v, %v, %v", judgment, accepted, err)
	}
	for _, change := range []func(*form.JudgmentResult){
		func(r *form.JudgmentResult) { r.InstructionID = "other" },
		func(r *form.JudgmentResult) { r.TargetVersion.Tree = "other" },
		func(r *form.JudgmentResult) { r.Norms = nil },
		func(r *form.JudgmentResult) { r.CompletionConditionIDs = []string{"other"} },
	} {
		changed := result
		change(&changed)
		data, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := AcceptResult(request, data, schema, provenance, nil); err == nil {
			t.Fatal("accepted a result that does not match the request")
		}
	}
	for _, tc := range []struct {
		name      string
		value     JudgmentValue
		findings  []Finding
		wantError bool
	}{
		{"PASS with blocking correction", Pass, []Finding{{ID: "F1", Blocking: true, Kind: AutoFixable, Scenario: "required correction"}}, true},
		{"PASS with blocking decision", Pass, []Finding{{ID: "F1", Blocking: true, Kind: NeedsDecision, Scenario: "required decision"}}, true},
		{"duplicate finding IDs", Pass, []Finding{result.Findings[0], result.Findings[0]}, true},
		{"FAIL without findings", Fail, []Finding{}, true},
		{"FAIL with reference information only", Fail, result.Findings, true},
		{"FAIL with blocking correction", Fail, []Finding{{ID: "F1", Blocking: true, Kind: AutoFixable, Scenario: "required correction"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := result
			changed.Judgment, changed.Findings = tc.value, tc.findings
			data, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			_, accepted, err := AcceptResult(request, data, schema, provenance, nil)
			if (err != nil) != tc.wantError || accepted == tc.wantError {
				t.Fatalf("acceptance: %v, %v; want error: %v", accepted, err, tc.wantError)
			}
		})
	}
	if _, _, err := AcceptResult(request, data, schema, Provenance{}, nil); err == nil {
		t.Fatal("accepted a result without provenance")
	}
	late, accepted, err := AcceptResult(request, []byte("invalid late arrival"), nil, Provenance{}, []Judgment{judgment})
	if err != nil || accepted || late.Judgment != Pass {
		t.Fatalf("late arrival changed the first judgment: %+v, %v", late, err)
	}
}

func TestFromCommand(t *testing.T) {
	request := judgmentRequest(t)
	request.Runner, request.Aspect.ID = "command:test", "command:test"
	var judgments []Judgment
	for _, command := range []CommandResult{{ExitCode: 0}, {ExitCode: 1, Output: "failure output"}, {ExitCode: 1, TimedOut: true}} {
		judgment, err := FromCommand(request, command)
		if err != nil {
			t.Fatal(err)
		}
		judgments = append(judgments, judgment)
	}
	if judgments[0].Judgment != Pass || judgments[1].Judgment != Fail || judgments[2].Judgment != NotAssessed {
		t.Fatalf("command judgments: %+v", judgments)
	}
}

func TestAggregate(t *testing.T) {
	judgments := []Judgment{
		{JudgmentResult: form.JudgmentResult{Judgment: Pass}},
		{JudgmentResult: form.JudgmentResult{Judgment: Fail}},
		{JudgmentResult: form.JudgmentResult{Judgment: NotAssessed}},
	}
	if Aggregate(judgments).Judgment != Fail || Aggregate([]Judgment{judgments[0], judgments[2]}).Judgment != NotAssessed || Aggregate(nil).Judgment != Pass {
		t.Fatal("unexpected aggregation priority")
	}
}

func TestApplyExemptions(t *testing.T) {
	judgment := Judgment{JudgmentResult: form.JudgmentResult{
		Judgment: Fail,
		Findings: []Finding{{
			ID: "F1", Blocking: true, Kind: AutoFixable, Scenario: "required correction",
			Exemption: &form.Exemption{Claim: "outside the agreed scope", Evidence: []form.Reference{{RecordID: "agreement"}}},
		}},
	}}
	decision := ExemptionDecision{FindingID: judgment.Findings[0].ID, ExcludedRequirements: []string{"out-of-scope requirement"}, Reason: "comparison against the agreed scope"}
	partial, err := ApplyExemptions(judgment, []ExemptionDecision{decision})
	if err != nil || partial.Judgment != Fail {
		t.Fatalf("partial exemption removed the entire violation: %+v, %v", partial, err)
	}
	decision.CoversFinding = true
	exempted, err := ApplyExemptions(judgment, []ExemptionDecision{decision})
	if err != nil || exempted.Judgment != Pass || exempted.Findings[0].Blocking || !judgment.Findings[0].Blocking {
		t.Fatalf("invalid exemption result or original value: %+v, %v", exempted, err)
	}
}

func TestCheckRequired(t *testing.T) {
	request := judgmentRequest(t)
	judgment := Judgment{JudgmentResult: form.JudgmentResult{
		Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion, Norms: request.normReferences(),
		CompletionConditionIDs: []string{"C1"}, Judgment: Pass,
	}, Runner: request.Runner}
	required := []Requirement{{Aspect: request.Aspect.ID, Runner: request.Runner, TargetVersion: request.TargetVersion, Norms: request.normReferences(), CompletionConditionIDs: []string{"C1", "C2"}}}
	if CheckRequired(required, []Judgment{judgment})[0].Judgment != NotAssessed {
		t.Fatal("marked an unchecked completion condition PASS")
	}
	if CheckRequired(required, nil)[0].Judgment != NotAssessed {
		t.Fatal("marked a required check without results PASS")
	}
}

func TestDetection(t *testing.T) {
	source := asset(t, "examples/purpose/spec.json")
	spec, err := ParseDetectionSpec(source)
	if err != nil {
		t.Fatal(err)
	}
	examples := make(map[string][]byte)
	var observations []DetectionObservation
	for _, example := range spec.Cases {
		examples[example.File] = asset(t, "examples/purpose/"+example.File)
		findings := []Finding{}
		if example.Kind == Violation {
			findings = append(findings, Finding{ID: "violation", Blocking: true, Kind: AutoFixable, Scenario: "completion cannot be determined"})
		}
		observations = append(observations, DetectionObservation{
			File: example.ID, Judgment: Judgment{JudgmentResult: form.JudgmentResult{Aspect: "purpose", Judgment: example.Expected, Findings: findings}, Runner: "review"},
			Reason: "compared the intended condition and result",
		})
	}
	references := map[string][]byte{}
	for _, name := range AspectReferences(string(asset(t, "checks/purpose.md"))) {
		references[name] = []byte("fixed norm content")
	}
	identity, err := NewDetectionIdentity(asset(t, "checks/purpose.md"), source, []byte("runner definition"), examples, references)
	if err != nil {
		t.Fatal(err)
	}
	record, err := AssessDetection(spec, "review", identity, observations)
	if err != nil || !record.Passed {
		t.Fatalf("detection test: %+v, %v", record, err)
	}
	observations[0].Reason = ""
	failed, err := AssessDetection(spec, "review", identity, observations)
	if err != nil || failed.Passed {
		t.Fatalf("FAIL without a reason passed: %+v, %v", failed, err)
	}
	required := []DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}}
	if len(MissingDetection(required, []DetectionTest{record})) != 0 {
		t.Fatal("could not reuse a passing record for identical content")
	}
	examples["allowed.md"] = append(examples["allowed.md"], '\n')
	changed, err := NewDetectionIdentity(asset(t, "checks/purpose.md"), source, []byte("runner definition"), examples, references)
	if err != nil {
		t.Fatal(err)
	}
	required[0].Identity = changed
	if identity == changed || len(MissingDetection(required, []DetectionTest{record})) != 1 {
		t.Fatal("changing an example did not invalidate the passing record")
	}
}
