package backlog

import (
	"embed"
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.json
var schema embed.FS

func Validate(data []byte) error {
	var value, definition any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	source, err := schema.ReadFile("schema.json")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(source, &definition); err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("https://devkit.invalid/backlog.json", definition); err != nil {
		return err
	}
	compiled, err := compiler.Compile("https://devkit.invalid/backlog.json")
	if err != nil {
		return err
	}
	return compiled.Validate(value)
}
