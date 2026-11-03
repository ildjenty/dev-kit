package form

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema は供給された JSON Schema を保持する。固定物の読み込みは行わない。
type Schema struct {
	compiled *jsonschema.Schema
}

// CompileSchema は供給されたスキーマをコンパイルする。
// 参照先は同じ JSON 内で定義し、ファイルやネットワークからは読み込まない。
func CompileSchema(source []byte) (*Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("スキーマの JSON: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	const location = "urn:devkit:form"
	if err := compiler.AddResource(location, doc); err != nil {
		return nil, fmt.Errorf("スキーマの登録: %w", err)
	}
	compiled, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("スキーマのコンパイル: %w", err)
	}
	return &Schema{compiled: compiled}, nil
}

// Validate は JSON の形式を検証する。受理条件や意味論の判定は行わない。
func (s *Schema) Validate(data []byte) error {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("入力の JSON: %w", err)
	}
	return s.compiled.Validate(value)
}

// Decode はスキーマで検証した JSON を dst にデコードする。
func (s *Schema) Decode(data []byte, dst any) error {
	if err := s.Validate(data); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
