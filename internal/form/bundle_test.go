package form

import (
	"bytes"
	"testing"
)

func TestBundleSchemaResolvesNestedDefinitions(t *testing.T) {
	source := []byte(`{"$defs":{"inner":{"$defs":{"value":{"$ref":"urn:test:value"}},"$ref":"#/$defs/inner/$defs/value"}},"$ref":"#/$defs/inner"}`)
	resources := map[string][]byte{"urn:test:value": []byte(`{"$id":"urn:test:value","type":"string"}`)}
	bundled, err := BundleSchema(source, resources)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := CompileSchema(bundled)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`"valid"`)); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`42`)); err == nil {
		t.Fatal("bundled schema lost the referenced type")
	}
	for i := 0; i < 5; i++ {
		again, err := BundleSchema(source, resources)
		if err != nil || !bytes.Equal(bundled, again) {
			t.Fatalf("bundle was not deterministic: %v", err)
		}
	}
	if _, err := BundleSchema(source, nil); err == nil {
		t.Fatal("missing reference was silently omitted")
	}
}

func TestBundleSchemaPreservesBooleanAndLocalAnchorSchemas(t *testing.T) {
	for _, source := range []string{`true`, `false`, `{"$defs":{"value":{"$anchor":"value","type":"string"}},"$ref":"#value"}`} {
		bundled, err := BundleSchema([]byte(source), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CompileSchema(bundled); err != nil {
			t.Fatal(err)
		}
	}
}
