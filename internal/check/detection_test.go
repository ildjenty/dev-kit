package check

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDetectionExamplesHideExpectations(t *testing.T) {
	spec, err := ParseDetectionSpec(asset(t, "examples/purpose/spec.json"))
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, example := range spec.Cases {
		bodies[example.File] = "Independent example body"
	}
	examples, err := PrepareDetectionExamples(spec, bodies)
	if err != nil {
		t.Fatal(err)
	}
	request := judgmentRequest(t)
	request.Kind, request.DetectionExamples = DetectionRequest, examples
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for i, example := range spec.Cases {
		if examples[i].ID != example.ID || examples[i].Body != bodies[example.File] || strings.Contains(string(data), example.File) {
			t.Fatalf("example mapping disclosed or lost private case information: %s", data)
		}
	}
	for _, secret := range []string{`"expected"`, `"violation"`, `"conforming"`, `"allowed"`} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("judge request disclosed %s", secret)
		}
	}
	data, err = json.Marshal(examples)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"kind"`) {
		t.Fatalf("judge examples disclosed case kinds: %s", data)
	}
	delete(bodies, spec.Cases[0].File)
	if _, err := PrepareDetectionExamples(spec, bodies); err == nil {
		t.Fatal("accepted a missing example body")
	}
}

func TestDetectionIdentityIncludesCriteriaTemplateAndRunner(t *testing.T) {
	source := asset(t, "examples/purpose/spec.json")
	spec, err := ParseDetectionSpec(source)
	if err != nil {
		t.Fatal(err)
	}
	examples := map[string][]byte{}
	for _, example := range spec.Cases {
		examples[example.File] = []byte("Example body")
	}
	criteria := []Criterion{{Source: "principles", ID: "review/purpose.md", ContentHash: "one"}, {Source: "principles", ID: "testing.md", ContentHash: "two"}}
	identity, err := NewDetectionIdentity([]byte("runner"), []byte("template"), criteria, source, examples)
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := NewDetectionIdentity([]byte("runner"), []byte("template"), []Criterion{criteria[1], criteria[0]}, source, examples)
	if err != nil || reordered != identity {
		t.Fatalf("criteria order changed the identity: %+v, %v", reordered, err)
	}
	changes := map[string]func() (DetectionIdentity, error){
		"criteria document": func() (DetectionIdentity, error) {
			changed := slices.Clone(criteria)
			changed[1].ContentHash = "changed"
			return NewDetectionIdentity([]byte("runner"), []byte("template"), changed, source, examples)
		},
		"criteria set": func() (DetectionIdentity, error) {
			return NewDetectionIdentity([]byte("runner"), []byte("template"), criteria[:1], source, examples)
		},
		"template": func() (DetectionIdentity, error) {
			return NewDetectionIdentity([]byte("runner"), []byte("template\n"), criteria, source, examples)
		},
		"runner": func() (DetectionIdentity, error) {
			return NewDetectionIdentity([]byte("runner\n"), []byte("template"), criteria, source, examples)
		},
	}
	for name, change := range changes {
		changed, err := change()
		if err != nil {
			t.Fatal(err)
		}
		required := []DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: changed}}
		records := []DetectionTest{{Aspect: "purpose", Runner: "review", Identity: identity, Passed: true}}
		if changed == identity || len(MissingDetection(required, records)) != 1 {
			t.Fatalf("changing the %s did not invalidate detection", name)
		}
	}
	if _, err := NewDetectionIdentity([]byte("runner"), []byte("template"), nil, source, examples); err == nil {
		t.Fatal("accepted a detection identity without criteria")
	}
	legacy := identity
	legacy.CriteriaHash = ""
	if len(MissingDetection([]DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}}, []DetectionTest{{Aspect: "purpose", Runner: "review", Identity: legacy, Passed: true}})) != 1 {
		t.Fatal("reused a passing record without criteria hashes")
	}
}

func TestUnregisteredCommand(t *testing.T) {
	request := judgmentRequest(t)
	request.Runner, request.Aspect.ID = "command:test", "command:test"
	judgment, err := UnregisteredCommand(request)
	if err != nil {
		t.Fatal(err)
	}
	if judgment.Judgment != Fail || len(judgment.Findings) != 1 || !judgment.Findings[0].Blocking || judgment.Findings[0].Kind != AutoFixable {
		t.Fatalf("missing command was not a fixable failure: %+v", judgment)
	}
	scenario := judgment.Findings[0].Scenario
	if !strings.Contains(scenario, "`test` is not registered") || !strings.Contains(scenario, "[commands.test]") || !strings.Contains(scenario, "timeout_seconds") || judgment.Command != nil || len(judgment.CompletionConditionIDs) != 0 || !judgment.ValidFor(request.TargetVersion, request.normReferences()) {
		t.Fatalf("incorrect missing command judgment: %+v", judgment)
	}
	request.Aspect.ID = "different"
	if _, err := UnregisteredCommand(request); err == nil {
		t.Fatal("accepted a mismatched command request")
	}
}
