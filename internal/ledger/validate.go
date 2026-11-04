package ledger

import (
	"encoding/json"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Validator applies the form and plan schemas supplied by the caller.
// It does not load kit assets or decide whether engine may accept a result.
type Validator struct {
	forms map[string]*form.Schema
	plans *plan.Validator
}

// NewValidator takes schemas keyed by filename without the .json extension.
// Required names are decision, evidence, progress, question, judgment, finding,
// reference, instruction, plan-template, and plan.
func NewValidator(sources map[string][]byte) (*Validator, error) {
	resources := make(map[string][]byte)
	for name, source := range sources {
		resources["urn:devkit:form:"+name] = source
	}
	v := &Validator{forms: make(map[string]*form.Schema)}
	provenance, err := form.CompileSchema([]byte(`{
		"type":"object",
		"properties":{
			"source":{"enum":["subagent","judge_command","command"]},
			"instruction_id":{"type":"string","minLength":1},
			"runner":{"type":"string","minLength":1},
			"agent_id":{"type":"string"},
			"effort":{"type":"string"},
			"model":{"type":"string"},
			"result_path":{"type":"string"}
		},
		"required":["source","instruction_id","runner"],
		"additionalProperties":false
	}`))
	if err != nil {
		return nil, fmt.Errorf("record schema provenance: %w", err)
	}
	v.forms["provenance"] = provenance
	for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "reference", "instruction"} {
		source, ok := sources[name]
		if !ok {
			return nil, fmt.Errorf("record schema %q is required", name)
		}
		schema, err := form.CompileSchemaWithResources(source, resources)
		if err != nil {
			return nil, fmt.Errorf("record schema %s: %w", name, err)
		}
		v.forms[name] = schema
	}
	for name, ref := range map[string]string{
		"target_version":  "judgment#/$defs/target_version",
		"norm_reference":  "judgment#/$defs/norm_reference",
		"approval_target": "question#/$defs/approval_target",
		"external_target": "question#/$defs/external_operation_target",
	} {
		source := []byte(`{"$ref":"urn:devkit:form:` + ref + `"}`)
		schema, err := form.CompileSchemaWithResources(source, resources)
		if err != nil {
			return nil, fmt.Errorf("record schema %s: %w", name, err)
		}
		v.forms[name] = schema
	}
	// Issuance records omit the regenerated instruction text and brief.
	var instruction map[string]json.RawMessage
	if err := json.Unmarshal(sources["instruction"], &instruction); err != nil {
		return nil, err
	}
	delete(instruction, "allOf")
	source, err := json.Marshal(instruction)
	if err != nil {
		return nil, err
	}
	v.forms["instruction"], err = form.CompileSchemaWithResources(source, resources)
	if err != nil {
		return nil, err
	}
	v.plans, err = plan.NewValidator(sources["plan-template"], sources["plan"])
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (v *Validator) check(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return v.forms[name].Validate(data)
}

// validateRaw preserves required-field checks before decoding supplies zero values.
func (v *Validator) validateRaw(payload Payload, data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	requiredFields := []string{}
	switch payload.(type) {
	case *Work:
		requiredFields = []string{"id", "task_ref", "branch", "base_commit", "kit_version"}
	case *Pause:
		requiredFields = []string{"position"}
	case *Resume:
		requiredFields = []string{"pause_id"}
	case *PlanVersion:
		requiredFields = []string{"plan", "origin"}
	case *Withdrawal:
		requiredFields = []string{"plan_version"}
	case *Lesson:
		requiredFields = []string{"content", "evidence"}
	case *Answer, *Approval:
		requiredFields = []string{"question_id", "target", "interpretation", "text", "original", "session_id", "observed_at"}
	case *ExternalOp:
		requiredFields = []string{"operation_id", "kind", "target", "input_hash", "definition_hash", "executable_hash", "approval_id", "input", "status"}
	case *Progress:
		requiredFields = []string{"round_id"}
	case *UnitResult:
		requiredFields = []string{"round_id", "unit_id", "agent_id", "result_path"}
	case *Merge:
		requiredFields = []string{"round_id", "head", "inputs", "applied", "current", "before_tree", "status"}
	case *Judgment:
		requiredFields = []string{"runner", "provenance"}
	case *ResultRejection:
		requiredFields = []string{"instruction_id", "result_path", "reason"}
	case *DetectionTest:
		requiredFields = []string{"instruction_id", "aspect", "runner", "identity", "cases", "passed"}
	case *StageRun:
		requiredFields = []string{"id", "stage_id", "plan_version", "state"}
	case *Round:
		requiredFields = []string{"id", "stage_run_id", "session_id", "start_target", "norms"}
	case *Instruction:
		requiredFields = []string{"id", "kind", "target_key", "reissue", "session_id"}
	case *Artifact:
		requiredFields = []string{"before", "after", "completion_condition_ids", "change_kind", "decisions"}
	}
	for _, name := range requiredFields {
		raw, ok := fields[name]
		if !ok || string(raw) == "null" {
			return fmt.Errorf("record field %q is required", name)
		}
	}
	schema := ""
	switch payload.(type) {
	case *Decision:
		schema = "decision"
	case *Evidence:
		schema = "evidence"
	case *Progress:
		schema = "progress"
		delete(fields, "round_id")
	case *Question:
		schema = "question"
		delete(fields, "supersedes")
	case *Judgment:
		schema = "judgment"
		if err := v.forms["provenance"].Validate(fields["provenance"]); err != nil {
			return fmt.Errorf("judgment provenance: %w", err)
		}
		for _, name := range []string{"round_id", "runner", "provenance", "content", "command"} {
			delete(fields, name)
		}
	case *PlanVersion:
		_, err := v.plans.ParsePlan(fields["plan"])
		return err
	}
	if schema == "" {
		return nil
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return v.forms[schema].Validate(data)
}

func required(fields ...string) error {
	for _, field := range fields {
		if field == "" {
			return fmt.Errorf("required record field is empty")
		}
	}
	return nil
}

func (v *Validator) answer(a Answer) error {
	if err := required(a.QuestionID, a.Original, a.SessionID); err != nil {
		return err
	}
	if a.ObservedAt.IsZero() {
		return fmt.Errorf("answer observation time is required")
	}
	switch a.Interpretation {
	case AnswerApproved, AnswerRejected, AnswerDeferred, AnswerBack:
	case AnswerText:
		if a.Text == "" {
			return fmt.Errorf("text answer requires text")
		}
	default:
		return fmt.Errorf("unknown answer interpretation: %q", a.Interpretation)
	}
	return v.check("approval_target", a.Target)
}

func (v *Validator) judgment(j Judgment) error {
	if err := required(j.Runner); err != nil {
		return err
	}
	if err := v.check("provenance", j.Provenance); err != nil {
		return fmt.Errorf("judgment provenance: %w", err)
	}
	if j.Provenance.InstructionID != j.InstructionID || j.Provenance.Runner != j.Runner {
		return fmt.Errorf("judgment provenance does not match instruction and runner")
	}
	return v.check("judgment", j.JudgmentResult)
}

func (v *Validator) validatePayload(payload Payload) error {
	switch p := payload.(type) {
	case *Work:
		return nil
	case *Pause:
		return v.check("reference", p.Position)
	case *Resume:
		return required(p.PauseID)
	case *PlanVersion:
		switch p.Origin {
		case plan.InitialProposal, plan.HumanProposal, plan.BackProposal:
		default:
			return fmt.Errorf("unknown plan proposal origin: %q", p.Origin)
		}
		return v.plans.ValidatePlan(p.Plan)
	case *Withdrawal:
		if p.PlanVersion <= 0 {
			return fmt.Errorf("withdrawn plan version must be positive")
		}
	case *Decision:
		return v.check("decision", p.Decision)
	case *Evidence:
		return v.check("evidence", p.Evidence)
	case *Lesson:
		if err := required(p.Content); err != nil {
			return err
		}
		if p.Evidence == nil {
			return fmt.Errorf("lesson evidence is required")
		}
	case *StateChange:
		return nil
	case *Question:
		return v.check("question", p.Question)
	case *Answer:
		return v.answer(*p)
	case *Approval:
		return v.answer(p.Answer)
	case *ExternalOp:
		return v.validateExternalOperation(p)
	case *Progress:
		if err := required(p.RoundID); err != nil {
			return err
		}
		return v.check("progress", p.Progress)
	case *UnitResult:
		if err := required(p.RoundID, p.UnitID, p.AgentID, p.ResultPath); err != nil {
			return err
		}
		return v.check("progress", p.Progress)
	case *Merge:
		return validateMerge(p)
	case *Judgment:
		return v.judgment(*p)
	case *ResultRejection:
		return required(p.InstructionID, p.ResultPath, p.Reason)
	case *DetectionTest:
		return v.validateDetectionTest(p)
	case *StageRun:
		return nil
	case *Round:
		return v.validateRound(p)
	case *Instruction:
		if err := required(p.SessionID); err != nil {
			return err
		}
		if p.PlanVersion < 0 {
			return fmt.Errorf("instruction plan version cannot be negative")
		}
		return v.check("instruction", form.Instruction{ID: p.ID, Kind: p.Kind, TargetKey: p.TargetKey, Reissue: p.Reissue})
	case *Artifact:
		if err := v.check("target_version", p.Before); err != nil {
			return err
		}
		if err := v.check("target_version", p.After); err != nil {
			return err
		}
		if p.CompletionConditionIDs == nil || p.Decisions == nil {
			return fmt.Errorf("artifact completion conditions and decisions are required")
		}
		return required(p.ChangeKind)
	}
	return nil
}

func (v *Validator) validateExternalOperation(p *ExternalOp) error {
	if err := v.check("external_target", p.ExternalOperationTarget); err != nil {
		return err
	}
	if err := required(p.ApprovalID); err != nil {
		return err
	}
	if len(p.Input) == 0 || !json.Valid(p.Input) {
		return fmt.Errorf("external operation input is required")
	}
	switch p.Status {
	case ExternalRunning:
	case ExternalSucceeded, ExternalFailed, ExternalUnknown:
		if p.BeforeID == "" {
			return fmt.Errorf("external operation result requires its before record")
		}
	default:
		return fmt.Errorf("unknown external operation status: %q", p.Status)
	}
	return nil
}

func (v *Validator) validateDetectionTest(p *DetectionTest) error {
	if err := required(p.InstructionID, p.Aspect, p.Runner, p.Identity.AspectHash, p.Identity.SpecHash, p.Identity.RunnerHash); err != nil {
		return err
	}
	if len(p.Cases) == 0 {
		return fmt.Errorf("detection test cases are required")
	}
	for _, result := range p.Cases {
		if err := required(result.File, result.Observation.File, result.Observation.Reason); err != nil {
			return err
		}
		switch result.Kind {
		case "violation", "conforming", "allowed":
		default:
			return fmt.Errorf("unknown detection case kind: %q", result.Kind)
		}
		switch result.Expected {
		case form.Pass, form.Fail, form.NotAssessed:
		default:
			return fmt.Errorf("unknown detection case judgment: %q", result.Expected)
		}
		if err := v.judgment(Judgment{Judgment: result.Observation.Judgment}); err != nil {
			return err
		}
	}
	return nil
}

func (v *Validator) validateRound(p *Round) error {
	if err := v.check("target_version", p.StartTarget); err != nil {
		return err
	}
	if p.ReturnedTarget != nil {
		if err := v.check("target_version", p.ReturnedTarget); err != nil {
			return err
		}
	}
	if p.Norms == nil {
		return fmt.Errorf("round norms are required")
	}
	for _, norm := range p.Norms {
		if err := v.check("norm_reference", norm); err != nil {
			return err
		}
	}
	if p.Decision != nil {
		switch p.Decision.Action {
		case plan.Fix, plan.Ask, plan.Back, plan.Done:
		default:
			return fmt.Errorf("unknown round action: %q", p.Decision.Action)
		}
	}
	return nil
}
