package record

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

//go:embed schemas/*.json
var schemas embed.FS

func schemaErrors(schema *jsonschema.Schema, value any) []string {
	errorsOut := []string{}
	err := schema.Validate(value)
	if err == nil {
		return errorsOut
	}
	var validation *jsonschema.ValidationError
	if !errors.As(err, &validation) {
		return []string{err.Error()}
	}
	var visit func(*jsonschema.ValidationError)
	visit = func(e *jsonschema.ValidationError) {
		path := ""
		for _, part := range e.InstanceLocation {
			path += "/" + strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
		}
		at := path
		if at == "" {
			at = "/"
		}
		add := func(message string) { errorsOut = append(errorsOut, at+" "+message) }
		switch k := e.ErrorKind.(type) {
		case *kind.Type:
			add("must be " + strings.Join(k.Want, " or "))
		case *kind.Enum:
			add("must be one of the allowed values")
		case *kind.Const:
			add("must equal the constant")
		case *kind.Pattern:
			add("must match " + k.Want)
		case *kind.MinLength:
			add(fmt.Sprintf("must have at least %d characters", k.Want))
		case *kind.MinItems:
			add(fmt.Sprintf("must have at least %d items", k.Want))
		case *kind.UniqueItems:
			add("must have unique items")
		case *kind.Minimum:
			n, _ := k.Want.Float64()
			add(fmt.Sprintf("must be >= %v", n))
		case *kind.Required:
			for _, key := range k.Missing {
				add("must have required property " + key)
			}
		case *kind.AdditionalProperties:
			for _, key := range k.Properties {
				errorsOut = append(errorsOut, path+"/"+strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")+" is an additional property")
			}
		case *kind.AnyOf:
			add("must match at least one alternative")
		case *kind.Not:
			add("must not match the forbidden schema")
		case *kind.FalseSchema:
			add("is not allowed")
		default:
			if len(e.Causes) == 0 {
				add(err.Error())
			} else {
				for _, cause := range e.Causes {
					visit(cause)
				}
			}
		}
	}
	visit(validation)
	return errorsOut
}

func ValidateSchema(file string, value any) ([]string, error) {
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(file)
	if err != nil {
		return nil, err
	}
	normalized, err := normalize(value)
	if err != nil {
		return []string{"/ must be a finite JSON value"}, nil
	}
	return schemaErrors(schema, normalized), nil
}
func normalize(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized any
	err = json.Unmarshal(data, &normalized)
	return normalized, err
}
func Schema(name string) (Document, error) {
	data, err := schemas.ReadFile("schemas/" + name + ".json")
	if err != nil {
		return nil, err
	}
	var definition Document
	if err = json.Unmarshal(data, &definition); err != nil {
		return nil, err
	}
	props := object(definition["properties"])
	switch name {
	case "plan":
		object(object(object(object(props["decisions"])["items"])["properties"])["quality"])["enum"] = concerns.Names()
	case "judgment":
		judge := object(props["judge"])
		judge["enum"] = append(concerns.Names(), stringsOf(judge["enum"])...)
	}
	value, err := normalize(definition)
	return object(value), err
}

func Validate(name string, value any) ([]string, error) {
	if _, err := schemas.ReadFile("schemas/" + name + ".json"); err != nil {
		return nil, fmt.Errorf("no schema for %s", name)
	}
	compiler := jsonschema.NewCompiler()
	files, err := fs.ReadDir(schemas, "schemas")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		doc, err := Schema(strings.TrimSuffix(file.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		if err := compiler.AddResource("https://devkit.invalid/schemas/"+file.Name(), doc); err != nil {
			return nil, err
		}
	}
	schema, err := compiler.Compile("https://devkit.invalid/schemas/" + name + ".json")
	if err != nil {
		return nil, err
	}
	normalized, err := normalize(value)
	if err != nil {
		return []string{"/ must be a finite JSON value"}, nil
	}
	problems := schemaErrors(schema, normalized)
	if len(problems) > 0 {
		return problems, nil
	}
	fields := map[string][]string{"requirements": {"completion_conditions", "open_points"}, "plan": {"decisions", "increments"}, "result": {"questions", "carried"}}
	doc, _ := normalized.(map[string]any)
	for _, field := range fields[name] {
		seen := map[string]bool{}
		items, _ := doc[field].([]any)
		for i, item := range items {
			id := item.(map[string]any)["id"].(string)
			if seen[id] {
				problems = append(problems, fmt.Sprintf("/%s/%d: id %s is used twice", field, i, id))
			}
			seen[id] = true
		}
	}
	return problems, nil
}

func WritePlanDraftSchema(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	schema, err := Schema("plan")
	if err != nil {
		return err
	}
	delete(schema, "$id")
	props := schema["properties"].(map[string]any)
	for _, key := range []string{"version", "requirements_version", "run_id", "recorded_at", "skeleton_commit", "derived_from"} {
		delete(props, key)
	}
	schema["required"] = []string{"decisions", "increments", "tests", "kept_behaviors"}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "draft.schema.json"), data, 0644); err != nil {
		return err
	}
	data, err = schemas.ReadFile("schemas/common.json")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "common.json"), data, 0644)
}
