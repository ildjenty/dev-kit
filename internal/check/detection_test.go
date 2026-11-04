package check

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestAspectReferences(t *testing.T) {
	source := "Use `principles/testing.md`, `rules.md`, and `principles/designing.md`.\n" +
		"Read `rules.md#screens` and `principles/anchored.md#test-cases`.\n" +
		"Repeat `principles/testing.md#coverage` and `rules.md#another-section`.\n" +
		"Repeat `principles/testing.md`; ignore principles/working.md and `checks/test.md`.\n" +
		"Ignore `principles/../secret.md` and `principles/bad\\name.md`."
	want := []string{"principles/anchored.md", "principles/designing.md", "principles/testing.md", "rules.md"}
	if got := AspectReferences(source); !slices.Equal(got, want) {
		t.Fatalf("references: %v; want: %v", got, want)
	}
}

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

func TestDetectionIdentityIncludesReferencedNorms(t *testing.T) {
	aspect := []byte("Read `principles/testing.md#coverage` and `rules.md#screens`.")
	source := asset(t, "examples/purpose/spec.json")
	spec, err := ParseDetectionSpec(source)
	if err != nil {
		t.Fatal(err)
	}
	examples := map[string][]byte{}
	for _, example := range spec.Cases {
		examples[example.File] = []byte("Example body")
	}
	norms := map[string][]byte{"principles/testing.md": []byte("Testing principle"), "rules.md": []byte("Repository rules")}
	identity, err := NewDetectionIdentity(aspect, source, []byte("runner"), examples, norms)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"principles/testing.md", "rules.md"} {
		original := norms[name]
		norms[name] = append(slices.Clone(original), '\n')
		changed, err := NewDetectionIdentity(aspect, source, []byte("runner"), examples, norms)
		if err != nil {
			t.Fatal(err)
		}
		required := []DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: changed}}
		records := []DetectionTest{{Aspect: "purpose", Runner: "review", Identity: identity, Passed: true}}
		if changed.NormsHash == identity.NormsHash || len(MissingDetection(required, records)) != 1 {
			t.Fatalf("changing %s did not invalidate detection", name)
		}
		norms[name] = original
	}
	norms["principles/unreferenced.md"] = []byte("Unrelated principle")
	unchanged, err := NewDetectionIdentity(aspect, source, []byte("runner"), examples, norms)
	if err != nil || unchanged != identity {
		t.Fatalf("unreferenced norm changed detection identity: %+v, %v", unchanged, err)
	}
	delete(norms, "rules.md")
	if _, err := NewDetectionIdentity(aspect, source, []byte("runner"), examples, norms); err == nil {
		t.Fatal("accepted missing referenced rules")
	}
	legacy := identity
	legacy.NormsHash = ""
	if len(MissingDetection([]DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}}, []DetectionTest{{Aspect: "purpose", Runner: "review", Identity: legacy, Passed: true}})) != 1 {
		t.Fatal("reused a passing record without norm hashes")
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
	want := "Verification command `test` is not registered. Create a verification method and register it in `[commands.test]` in `devkit.toml`."
	if judgment.Findings[0].Scenario != want || judgment.Command != nil || len(judgment.CompletionConditionIDs) != 0 || !judgment.ValidFor(request.TargetVersion, request.normReferences()) {
		t.Fatalf("incorrect missing command judgment: %+v", judgment)
	}
	request.Aspect.ID = "different"
	if _, err := UnregisteredCommand(request); err == nil {
		t.Fatal("accepted a mismatched command request")
	}
}
