package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator は kit が供給したスキーマを保持し、ファイルは読まない。
type Validator struct {
	template *jsonschema.Schema
	plan     *jsonschema.Schema
}

// NewValidator には埋め込みからも取得できる2つのスキーマの内容を渡す。
func NewValidator(templateSchema, planSchema []byte) (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	for _, resource := range []struct {
		path string
		data []byte
	}{{TemplateSchemaPath, templateSchema}, {PlanSchemaPath, planSchema}} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(resource.data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", resource.path, err)
		}
		if err := compiler.AddResource(schemaURL(resource.path), doc); err != nil {
			return nil, fmt.Errorf("%s: %w", resource.path, err)
		}
	}
	template, err := compiler.Compile(schemaURL(TemplateSchemaPath))
	if err != nil {
		return nil, fmt.Errorf("雛形のスキーマ: %w", err)
	}
	plan, err := compiler.Compile(schemaURL(PlanSchemaPath))
	if err != nil {
		return nil, fmt.Errorf("計画のスキーマ: %w", err)
	}
	return &Validator{template: template, plan: plan}, nil
}

func schemaURL(path string) string {
	return "https://devkit.invalid/schemas/v1/" + strings.TrimPrefix(path, "kit/schemas/")
}

func decode(data []byte, schema *jsonschema.Schema, result any) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("JSON の読み込み: %w", err)
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("JSON Schema の検証: %w", err)
	}
	return json.Unmarshal(data, result)
}

func (v *Validator) ParseTemplate(data []byte) (PlanTemplate, error) {
	var template PlanTemplate
	if err := decode(data, v.template, &template); err != nil {
		return PlanTemplate{}, err
	}
	if err := validateStages(template.StartStage, template.Stages, template.OptionalStages, template.MetaLoopStages, nil); err != nil {
		return PlanTemplate{}, err
	}
	return template, nil
}

func (v *Validator) ParsePlan(data []byte) (Plan, error) {
	var plan Plan
	if err := decode(data, v.plan, &plan); err != nil {
		return Plan{}, err
	}
	conditions := make(map[string]bool)
	for _, condition := range plan.CompletionConditions {
		if conditions[condition.ID] {
			return Plan{}, fmt.Errorf("完了条件 ID の重複: %q", condition.ID)
		}
		conditions[condition.ID] = true
	}
	if err := validateStages(plan.StartStage, plan.Stages, nil, plan.MetaLoopStages, conditions); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func (v *Validator) ValidateTemplate(template PlanTemplate) error {
	data, err := json.Marshal(template)
	if err != nil {
		return err
	}
	_, err = v.ParseTemplate(data)
	return err
}

func (v *Validator) ValidatePlan(plan Plan) error {
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	_, err = v.ParsePlan(data)
	return err
}

func validateStages(start string, stages, optional []Stage, meta []string, conditions map[string]bool) error {
	all := append(slices.Clone(stages), optional...)
	ids := make(map[string]bool)
	for _, stage := range all {
		if ids[stage.ID] {
			return fmt.Errorf("ステージ ID の重複: %q", stage.ID)
		}
		ids[stage.ID] = true
	}
	if !slices.ContainsFunc(stages, func(s Stage) bool { return s.ID == start }) {
		return fmt.Errorf("開始ステージが採用ステージにない: %q", start)
	}
	for _, id := range meta {
		if !ids[id] {
			return fmt.Errorf("メタループのステージがない: %q", id)
		}
	}
	for _, stage := range all {
		for _, action := range []Transition{Fix, Ask, Back, Done} {
			target, _ := stage.Transitions.Next(action)
			if target != nil && !ids[*target] {
				return fmt.Errorf("%s の %s の遷移先がない: %q", stage.ID, action, *target)
			}
			if target != nil && slices.ContainsFunc(stages, func(s Stage) bool { return s.ID == stage.ID }) &&
				!slices.ContainsFunc(stages, func(s Stage) bool { return s.ID == *target }) {
				return fmt.Errorf("%s の %s が未採用の候補を参照している: %q", stage.ID, action, *target)
			}
		}
		for _, check := range stage.Checks {
			if check.Runner == "command:" {
				return fmt.Errorf("%s: command の名前がない", stage.ID)
			}
			if (check.Runner.IsCommand() || strings.HasPrefix(check.Aspect, "command:")) && check.Aspect != string(check.Runner) {
				return fmt.Errorf("%s: command の観点と runner が異なる", stage.ID)
			}
			for _, id := range check.CompletionConditionIDs {
				if conditions != nil && !conditions[id] {
					return fmt.Errorf("%s: 完了条件がない: %q", stage.ID, id)
				}
			}
		}
	}
	return nil
}
