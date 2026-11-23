package retro

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
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

func schema(name string, value any) ([]string, error) {
	compiler := jsonschema.NewCompiler()
	data, err := schemas.ReadFile("schemas/" + name + ".schema.json")
	if err != nil {
		return nil, err
	}
	var source any
	if err = json.Unmarshal(data, &source); err != nil {
		return nil, err
	}
	url := "https://devkit.invalid/retro/" + name + ".json"
	if err = compiler.AddResource(url, source); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}
	data, err = json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized any
	if err = json.Unmarshal(data, &normalized); err != nil {
		return nil, err
	}
	return schemaErrors(compiled, normalized), nil
}
