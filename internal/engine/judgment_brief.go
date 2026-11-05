package engine

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// requestView is the machine-readable part of a judgment brief: identities and hashes only.
type requestView struct {
	InstructionID          string               `json:"instruction_id"`
	Kind                   check.RequestKind    `json:"kind"`
	Runner                 string               `json:"runner"`
	Aspect                 string               `json:"aspect"`
	Criteria               []check.Criterion    `json:"criteria"`
	TargetVersion          form.TargetVersion   `json:"target_version"`
	TargetPath             string               `json:"target_path"`
	Norms                  []form.NormReference `json:"norms"`
	CompletionConditionIDs []string             `json:"completion_condition_ids"`
	PreviousFindingIDs     []string             `json:"previous_finding_ids"`
}

func viewRequest(request check.Request) requestView {
	return requestView{InstructionID: request.InstructionID, Kind: request.Kind, Runner: request.Runner, Aspect: request.Aspect.ID, Criteria: request.Criteria,
		TargetVersion: request.TargetVersion, TargetPath: request.TargetPath, Norms: normRefs(request.Norms), CompletionConditionIDs: request.CompletionConditionIDs, PreviousFindingIDs: request.PreviousFindingIDs}
}

// judgmentContext fixes the judge's inputs at issue time: adopted outputs selected by the judged stage,
// current material with the always-required evidence, plan and pre-judgment instructions,
// standing delegations with their sources, input schemas, and the history index.
func (e *Engine) judgmentContext(ctx context.Context, state *ledger.State, item catalogCheck, meta *ledger.Instruction, request check.Request, round *ledger.Round) error {
	if meta.Context != nil && meta.Context.CurrentMaterial != "" {
		return nil
	}
	material, err := e.judgmentMaterial(ctx, state, request, round)
	if err != nil {
		return err
	}
	adopted, err := e.adoptedOutputs(state)
	if err != nil {
		return err
	}
	if meta.Context == nil {
		meta.Context = &ledger.InstructionContext{}
	}
	meta.Context.CurrentMaterial = material + "\n"
	var selection []string
	if round != nil {
		stage, _, err := e.roundPlanStage(state, round)
		if err != nil {
			return err
		}
		selection = stage.Outputs
	}
	meta.Context.Outputs = contextOutputs(selectOutputs(adopted, selection))
	meta.Context.Schemas, meta.Context.Instructions = item.Schemas, item.Instructions
	scope := historyScope{Kind: item.JudgmentHistory(), Aspect: item.Aspect, Runner: string(item.Runner)}
	if round != nil {
		scope.RunID = round.StageRunID
		for _, planned := range state.JudgmentBriefs(round.ID) {
			if planned.Aspect == item.Aspect && planned.Runner == string(item.Runner) {
				meta.Context.PreJudgment = planned.Instructions
			}
		}
		if meta.Context.Delegations, err = e.delegations(ctx, state, round); err != nil {
			return err
		}
	}
	_, err = e.historyIndex(state, meta, scope, adopted)
	return err
}

// delegations returns the stage's selected delegation documents verbatim with their sources.
func (e *Engine) delegations(ctx context.Context, state *ledger.State, round *ledger.Round) ([]ledger.InstructionOutput, error) {
	stage, _, err := e.roundPlanStage(state, round)
	if err != nil || len(stage.Delegations) == 0 {
		return nil, err
	}
	documents, err := e.selectionDocuments(ctx, roundReference(round), roundReference(round), stage.Delegations)
	if err != nil {
		return nil, err
	}
	result := []ledger.InstructionOutput{}
	for _, document := range documents {
		result = append(result, ledger.InstructionOutput{StageID: document.Path, Content: document.Content})
	}
	return result, nil
}

// roundPlanStage returns the bound plan stage of a round's StageRun.
func (e *Engine) roundPlanStage(state *ledger.State, round *ledger.Round) (plan.Stage, plan.Plan, error) {
	for _, run := range state.StageRuns() {
		if run.ID != round.StageRunID {
			continue
		}
		p, ok, err := e.boundPlan(run.PlanVersion)
		if err != nil {
			return plan.Stage{}, plan.Plan{}, err
		}
		if !ok {
			return plan.Stage{}, plan.Plan{}, errors.New("bound plan is missing")
		}
		stage, _ := p.Stage(run.StageID)
		return stage, p, nil
	}
	return plan.Stage{}, plan.Plan{}, errors.New("round stage run is missing")
}

func (e *Engine) judgmentBrief(ctx context.Context, data instructionData) (brief.Brief, error) {
	input, err := e.judgmentInput(ctx, data)
	if err != nil {
		return brief.Brief{}, err
	}
	settings, err := e.settings(ctx, data.Reference)
	if err != nil {
		return brief.Brief{}, err
	}
	meta := &ledger.Instruction{ID: data.Instruction.ID, Context: data.Context}
	return e.fitBrief(meta, settings.BriefMaxBytes(), judgmentSpills(&input), func() (brief.Brief, error) { return brief.BuildJudgment(input) })
}

func (e *Engine) judgmentInput(ctx context.Context, data instructionData) (brief.JudgmentInput, error) {
	template, err := e.asset(ctx, data.Reference, "templates/brief/judgment.md.tmpl")
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	productRef := data.Reference
	if data.Request.Kind == check.RoundRequest {
		productRef = data.Request.TargetVersion.Tree
	}
	facts, err := e.repository(ctx, data.Reference, data.Inputs, productRef)
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	criteria, err := e.criteriaDocuments(ctx, data.Reference, data.Request.Criteria)
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	paths, err := e.normPaths(ctx, data.Reference)
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	content, err := json.MarshalIndent(viewRequest(*data.Request), "", "  ")
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	schema, err := e.judgmentSchema(ctx, data.Reference, data.Request.Runner)
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	docs, err := e.additionalInputs(ctx, data.Reference, productRef, data.Inputs)
	if err != nil {
		return brief.JudgmentInput{}, err
	}
	// A document already supplied as a criterion is not repeated among the norms.
	docs = slices.DeleteFunc(docs, func(doc brief.Document) bool {
		return slices.ContainsFunc(criteria, func(criterion brief.Document) bool { return criterion.Path == doc.Path })
	})
	input := brief.JudgmentInput{RequestTemplate: string(template), InstructionID: data.Instruction.ID, Content: "```json\n" + string(content) + "\n```", Criteria: criteria, Repository: facts, Norms: docs, NormPaths: paths, OutputSchema: string(schema)}
	return input, e.judgmentInputContext(ctx, data, &input)
}

func (e *Engine) judgmentInputContext(ctx context.Context, data instructionData, input *brief.JudgmentInput) error {
	if data.Context == nil {
		return errors.New("judgment instruction context is missing")
	}
	var err error
	if input.SupportingSchemas, err = e.inputSchemas(ctx, data.Reference, data.Context.Schemas); err != nil {
		return err
	}
	if data.Request.Kind == check.RoundRequest {
		documents, err := e.roundDocuments(ctx, data.RoundID)
		if err != nil {
			return err
		}
		input.Norms = append(input.Norms, documents...)
	}
	input.Instructions, input.PreJudgmentInstructions = data.Context.Instructions, data.Context.PreJudgment
	input.PreviousOutputs, input.CurrentMaterial = instructionOutputs(data.Context), data.Context.CurrentMaterial
	for _, delegation := range data.Context.Delegations {
		input.Delegations = append(input.Delegations, brief.Document{Path: delegation.StageID, Content: delegation.Content, Exists: true})
	}
	name := "brief/" + data.Instruction.ID + "-history.md"
	if err := e.store().Publish(name, []byte(data.Context.WorkHistory)); err != nil {
		return err
	}
	input.WorkHistoryPath = filepath.Join(e.workRoot(), name)
	return nil
}

// roundDocuments returns the agreed plan and the stage definition of a round judgment.
func (e *Engine) roundDocuments(ctx context.Context, roundID string) ([]brief.Document, error) {
	state, err := e.load()
	if err != nil {
		return nil, err
	}
	for _, round := range allRounds(state) {
		if round.ID != roundID {
			continue
		}
		stage, p, err := e.roundPlanStage(state, round)
		if err != nil {
			return nil, err
		}
		agreed, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return nil, err
		}
		definition, err := e.asset(ctx, roundReference(round), "stages/"+stage.DefinitionID()+".md")
		if err != nil {
			return nil, err
		}
		return []brief.Document{{Path: "Agreed plan", Content: "```json\n" + string(agreed) + "\n```", Exists: true}, {Path: "Stage definition stages/" + stage.DefinitionID() + ".md", Content: string(definition), Exists: true}}, nil
	}
	return nil, errors.New("judgment round is missing")
}

func allRounds(state *ledger.State) []*ledger.Round {
	rounds := []*ledger.Round{}
	for _, run := range state.StageRuns() {
		rounds = append(rounds, state.Rounds(run.ID)...)
	}
	return rounds
}

func judgmentSpills(input *brief.JudgmentInput) []spill {
	items := []spill{{"Request", &input.Content}, {"Output schema", &input.OutputSchema}, {"Current material", &input.CurrentMaterial}, {"Rules", &input.Repository.Rules.Content}, {"Requirements", &input.Repository.Requirements.Content}}
	for i := range input.Criteria {
		items = append(items, spill{"Criterion " + input.Criteria[i].Path, &input.Criteria[i].Content})
	}
	for i := range input.PreviousOutputs {
		items = append(items, spill{"Adopted output " + input.PreviousOutputs[i].StageID, &input.PreviousOutputs[i].Content})
	}
	for i := range input.Norms {
		items = append(items, spill{"Norm " + input.Norms[i].Path, &input.Norms[i].Content})
	}
	for i := range input.Repository.LayoutGuides {
		items = append(items, spill{"Structure record " + input.Repository.LayoutGuides[i].Path, &input.Repository.LayoutGuides[i].Content})
	}
	for i := range input.Delegations {
		items = append(items, spill{"Delegation " + input.Delegations[i].Path, &input.Delegations[i].Content})
	}
	for i := range input.SupportingSchemas {
		items = append(items, spill{"Schema " + input.SupportingSchemas[i].Path, &input.SupportingSchemas[i].Content})
	}
	return items
}
