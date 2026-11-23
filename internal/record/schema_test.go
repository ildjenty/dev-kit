package record

import (
	"bytes"
	"encoding/json"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func checkSchema(t *testing.T, schema, value any) []string {
	file := filepath.Join(t.TempDir(), "schema.json")
	testutil.Put(t, filepath.Dir(file), "schema.json", string(must(json.Marshal(schema))))
	return must(ValidateSchema(file, value))
}
func TestSchemaTypes(t *testing.T) {
	for _, tc := range []struct {
		kind      string
		good, bad any
	}{{"object", Document{}, []any{}}, {"array", []any{}, Document{}}, {"null", nil, Document{}}, {"integer", 1, 1.5}, {"number", 1.5, math.Inf(1)}, {"boolean", true, 1}, {"string", "", nil}} {
		t.Run(tc.kind, func(t *testing.T) {
			testutil.Equal(t, checkSchema(t, Document{"type": tc.kind}, tc.good), []string{})
			if len(checkSchema(t, Document{"type": tc.kind}, tc.bad)) == 0 {
				t.Fatal("expected invalid value")
			}
		})
	}
	testutil.Equal(t, checkSchema(t, Document{"type": []string{"string", "null"}}, nil), []string{})
	testutil.Equal(t, checkSchema(t, true, 1), []string{})
	if len(checkSchema(t, false, 1)) == 0 {
		t.Fatal("false schema accepted value")
	}
}
func TestSchemaProperties(t *testing.T) {
	schema := Document{"type": "object", "properties": Document{"n": Document{"type": "number", "minimum": 1}}, "required": []string{"n"}, "additionalProperties": false}
	testutil.Equal(t, checkSchema(t, schema, Document{"n": 1}), []string{})
	for _, value := range []any{Document{}, Document{"n": 0}, Document{"n": "1"}, Document{"n": 1, "extra": true}, Document{}} {
		if len(checkSchema(t, schema, value)) == 0 {
			t.Fatalf("accepted %v", value)
		}
	}
	if len(checkSchema(t, Document{"additionalProperties": Document{"type": "string"}}, Document{"n": 1})) == 0 {
		t.Fatal("accepted additional number")
	}
}
func TestSchemaConstraints(t *testing.T) {
	testutil.Equal(t, checkSchema(t, Document{"type": "string", "minLength": 1}, "😀"), []string{})
	if len(checkSchema(t, Document{"type": "string", "minLength": 1}, "")) == 0 {
		t.Fatal("accepted empty")
	}
	if len(checkSchema(t, Document{"pattern": "^Q[1-9][0-9]*$"}, "Q0")) == 0 {
		t.Fatal("accepted Q0")
	}
	schema := Document{"type": "array", "minItems": 1, "uniqueItems": true, "items": Document{"enum": []string{"a", "b"}}}
	testutil.Equal(t, checkSchema(t, schema, []string{"a", "b"}), []string{})
	for _, value := range []any{[]string{}, []string{"a", "a"}, []string{"c"}} {
		if len(checkSchema(t, schema, value)) == 0 {
			t.Fatalf("accepted %v", value)
		}
	}
	if len(checkSchema(t, Document{"uniqueItems": true}, []Document{{"a": 1, "b": 2}, {"b": 2, "a": 1}})) == 0 {
		t.Fatal("accepted duplicate objects")
	}
	testutil.Equal(t, checkSchema(t, Document{"const": Document{"a": 1}}, Document{"a": 1}), []string{})
	if len(checkSchema(t, Document{"const": false}, true)) == 0 {
		t.Fatal("accepted wrong const")
	}
}
func TestSchemaReferences(t *testing.T) {
	dir := t.TempDir()
	testutil.Put(t, dir, "common.json", `{"$defs":{"text":{"type":"string","minLength":1},"alias":{"$ref":"#/$defs/text"}}}`)
	testutil.Put(t, dir, "root.json", `{"type":"object","properties":{"value":{"$ref":"common.json#/$defs/alias"}}}`)
	testutil.Equal(t, must(ValidateSchema(filepath.Join(dir, "root.json"), Document{"value": "yes"})), []string{})
	if len(must(ValidateSchema(filepath.Join(dir, "root.json"), Document{"value": ""}))) == 0 {
		t.Fatal("accepted empty reference")
	}
}
func TestSchemaCompositions(t *testing.T) {
	schema := Document{"allOf": []Document{{"type": "object"}, {"properties": Document{"kind": Document{"enum": []string{"a", "b"}}}, "required": []string{"kind"}}}, "if": Document{"properties": Document{"kind": Document{"const": "a"}}}, "then": Document{"required": []string{"value"}}, "else": Document{"not": Document{"required": []string{"value"}}}}
	for _, value := range []Document{{"kind": "a", "value": 1}, {"kind": "b"}} {
		testutil.Equal(t, checkSchema(t, schema, value), []string{})
	}
	for _, value := range []Document{{"kind": "a"}, {"kind": "b", "value": 1}, {"kind": "c"}} {
		if len(checkSchema(t, schema, value)) == 0 {
			t.Fatalf("accepted %v", value)
		}
	}
	anyOf := Document{"anyOf": []Document{{"type": "string"}, {"type": "number"}}}
	testutil.Equal(t, checkSchema(t, anyOf, 1), []string{})
	if len(checkSchema(t, anyOf, nil)) == 0 {
		t.Fatal("accepted null")
	}
}
func TestRecordSchemaConditionsAndDuplicateIDs(t *testing.T) {
	doc := Document{"run_id": "r1", "workflow": "refine", "recorded_at": "t", "outcome": "pass", "inputs": Document{"requirements_version": nil, "plan_version": nil}, "base": sha("a"), "commit": sha("b"), "criteria_commit": sha("c"), "loops": []any{}}
	testutil.Equal(t, must(Validate("result", doc)), []string{})
	q := Document{"id": "Q1", "text": "Choose", "target": Document{"kind": "decision", "ref": "D1"}, "next_workflows": []string{"refine"}, "invalidates": []any{}}
	for _, extra := range []Document{{"stop": Document{"state": "stopped", "unresolved": []any{}, "premise": ""}}, {"outcome": "question"}, {"outcome": "question", "questions": []Document{q, q}}, {"outcome": "question", "questions": []Document{{"id": "Q1", "text": "Choose", "target": q["target"], "next_workflows": []string{"refine", "refine"}, "invalidates": []any{}}}}, {"outcome": "carry", "carried": []Document{{"id": "F1", "judge": "data", "text": "Finding"}}}} {
		value := clone(doc)
		for k, v := range extra {
			value[k] = v
		}
		if len(must(Validate("result", value))) == 0 {
			t.Fatalf("accepted %v", value)
		}
	}
	for _, kind := range []string{"requirements", "plan"} {
		dir := newRecord(t)
		var value Document
		if kind == "requirements" {
			value = requirements(dir).Entry
		} else {
			value = plan(dir).Entry
		}
		field := "completion_conditions"
		if kind == "plan" {
			field = "increments"
		}
		entries := array(value[field])
		value[field] = []any{entries[0], entries[0]}
		testutil.Contains(t, must(Validate(kind, value))[0], "used twice")
	}
}

func TestDownstreamResultsCannotAskTheOwner(t *testing.T) {
	for _, workflow := range []string{"implement", "judge", "fix"} {
		value := Document{"run_id": "r1", "workflow": workflow, "recorded_at": "t", "outcome": "question", "inputs": Document{"requirements_version": 1, "plan_version": 1}, "base": sha("a"), "commit": sha("b"), "criteria_commit": sha("c"), "loops": []Document{}, "questions": []Document{{"id": "Q1", "text": "Choose audience", "target": Document{"kind": "decision", "ref": "D9"}, "next_workflows": []string{"plan"}, "invalidates": []int{}}}}
		if len(must(Validate("result", value))) == 0 {
			t.Fatalf("%s can still ask the owner", workflow)
		}
	}
}

func TestDeferredFindingSchemaRequiresNonblockingNonOwnerFinding(t *testing.T) {
	f := Document{"id": "structure-1", "blocking": false, "needs_decision": false, "deferred": true, "mechanical": true, "text": "Deferred smell", "evidence": "elsewhere"}
	j := Document{"run_id": "j1", "recorded_at": "t", "judge": "structure", "kind": "code", "target": Document{"base": sha("a"), "commit": sha("b")}, "criteria_commit": sha("c"), "materials": []string{}, "observations": []string{}, "verdict": "pass", "findings": []Document{f}}
	testutil.Equal(t, must(Validate("judgment", j)), []string{})
	for _, field := range []string{"blocking", "needs_decision"} {
		f[field] = true
		if len(must(Validate("judgment", j))) == 0 {
			t.Fatal("deferred finding can still", field)
		}
		f[field] = false
	}
}

func TestRefineJudgmentPhaseIsRestrictedToRefineProposals(t *testing.T) {
	doc := Document{"run_id": "r1", "recorded_at": "t", "judge": "refine", "kind": "proposal", "phase": "decide", "target": Document{"proposal_version": 1}, "criteria_commit": sha("c"), "materials": []any{}, "observations": []any{}, "verdict": "pass", "findings": []any{}}
	testutil.Equal(t, must(Validate("judgment", doc)), []string{})
	doc["phase"] = "document"
	testutil.Equal(t, must(Validate("judgment", doc)), []string{})
	doc["judge"] = "structure"
	if len(must(Validate("judgment", doc))) == 0 {
		t.Fatal("accepted refine phase for a quality judge")
	}
}

func TestF17EmbeddedQualityEnumsAreLoadedFromConcerns(t *testing.T) {
	for _, name := range []string{"plan", "judgment"} {
		data := must(schemas.ReadFile("schemas/" + name + ".json"))
		if bytes.Contains(data, []byte(`"readability"`)) {
			t.Fatalf("%s embeds a second quality list", name)
		}
	}
	dir := t.TempDir()
	testutil.OK(t, WritePlanDraftSchema(dir))
	var schema Document
	testutil.OK(t, json.Unmarshal(must(os.ReadFile(filepath.Join(dir, "draft.schema.json"))), &schema))
	enum := object(object(object(object(schema["properties"])["decisions"])["items"])["properties"])["quality"]
	testutil.Equal(t, object(enum)["enum"], concerns.Names())
}
