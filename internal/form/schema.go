package form

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema holds a supplied JSON Schema. It does not load assets.
type Schema struct {
	compiled *jsonschema.Schema
}

// CompileSchema compiles a supplied schema.
// References must be defined in the same JSON; files and network resources are not loaded.
func CompileSchema(source []byte) (*Schema, error) {
	return CompileSchemaWithResources(source, nil)
}

// CompileSchemaWithResources compiles a schema with supplied referenced schemas.
// Resource keys are absolute schema URIs. Files and network resources are not loaded.
func CompileSchemaWithResources(source []byte, resources map[string][]byte) (*Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("Schema JSON: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	const location = "urn:devkit:form"
	if err := compiler.AddResource(location, doc); err != nil {
		return nil, fmt.Errorf("Schema registration: %w", err)
	}
	for uri, source := range resources {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(source))
		if err != nil {
			return nil, fmt.Errorf("Schema JSON %q: %w", uri, err)
		}
		if err := compiler.AddResource(uri, doc); err != nil {
			return nil, fmt.Errorf("Schema registration %q: %w", uri, err)
		}
	}
	compiled, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("Schema compilation: %w", err)
	}
	return &Schema{compiled: compiled}, nil
}

// Validate checks the JSON format. It does not evaluate acceptance conditions or semantics.
func (s *Schema) Validate(data []byte) error {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("Input JSON: %w", err)
	}
	return s.compiled.Validate(value)
}

// Decode validates JSON against the schema and decodes it into dst.
func (s *Schema) Decode(data []byte, dst any) error {
	if err := s.Validate(data); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
