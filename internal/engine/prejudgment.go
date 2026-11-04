package engine

import (
	"context"
	"errors"
	"strings"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func hasJudgmentBrief(state *ledger.State, round *ledger.Round, item plan.Check) bool {
	for _, planned := range state.JudgmentBriefs(round.ID) {
		if planned.Aspect == item.Aspect && planned.Runner == string(item.Runner) {
			return true
		}
	}
	return false
}

// judgmentBriefInstruction asks the planner how to check this round before a brief_by_planner
// judgment. It uses the same round, aspect, and runner key as the judgment.
func (e *Engine) judgmentBriefInstruction(ctx context.Context, state *ledger.State, p plan.Plan, round *ledger.Round, item plan.Check) (Result, error) {
	meta, fresh, err := e.issuance(form.InstructionJudgmentBrief, judgmentKey(round, item), round, p.Version, "")
	if err != nil {
		return Result{}, err
	}
	if !fresh {
		old, err := e.readInstruction(meta.ID)
		if err != nil {
			return Result{}, err
		}
		return e.publishInstruction(meta, false, old, "")
	}
	b, err := e.judgmentBriefText(ctx, state, p, round, item, &meta)
	if err != nil {
		return Result{}, err
	}
	ref := roundReference(round)
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return Result{}, err
	}
	runner, err := e.runnerAgent(ctx, ref, settings.PlanningRole())
	if err != nil {
		return Result{}, err
	}
	data := instructionData{Reference: ref, Instruction: form.Instruction{Runner: runner, Text: "Launch the planner with: devkit-instruction " + meta.ID}}
	return e.publishInstruction(meta, true, data, b.Text)
}

func (e *Engine) judgmentBriefText(ctx context.Context, state *ledger.State, p plan.Plan, round *ledger.Round, item plan.Check, meta *ledger.Instruction) (brief.Brief, error) {
	ref := roundReference(round)
	template, err := e.asset(ctx, ref, "templates/brief/judgment-brief.md.tmpl")
	if err != nil {
		return brief.Brief{}, err
	}
	schema, err := e.asset(ctx, ref, "schemas/judgment-brief.json")
	if err != nil {
		return brief.Brief{}, err
	}
	facts, err := e.repository(ctx, ref, []plan.Selection{{Source: "requirements"}}, round.ReturnedTarget.Tree)
	if err != nil {
		return brief.Brief{}, err
	}
	if meta.Context == nil {
		records, err := e.records()
		if err != nil {
			return brief.Brief{}, err
		}
		request := check.Request{}
		if progress := roundProgress(records, round.ID); progress != nil {
			request.Progress = &progress.Progress
		}
		material, err := e.judgmentMaterial(ctx, state, request, round)
		if err != nil {
			return brief.Brief{}, err
		}
		stage, _, err := e.roundPlanStage(state, round)
		if err != nil {
			return brief.Brief{}, err
		}
		main, err := e.mainInstructions("planner", stage.ID)
		if err != nil {
			return brief.Brief{}, err
		}
		meta.Context = &ledger.InstructionContext{MainInstructions: main, CurrentMaterial: material}
	}
	body, err := brief.BuildJudgmentBrief(brief.JudgmentBriefInput{RequestTemplate: string(template), InstructionID: meta.ID, Check: item, Plan: p, Requirements: facts.Requirements, CurrentMaterial: meta.Context.CurrentMaterial, OutputSchema: string(schema)})
	return mainInstructionBrief(body, meta.Context.MainInstructions), err
}

func (e *Engine) regenerateJudgmentBrief(ctx context.Context, state *ledger.State, meta ledger.Instruction) (brief.Brief, error) {
	for _, run := range state.StageRuns() {
		for _, round := range state.Rounds(run.ID) {
			if round.ID != meta.RoundID {
				continue
			}
			stage, p, err := e.roundPlanStage(state, round)
			if err != nil {
				return brief.Brief{}, err
			}
			for _, item := range stage.Checks {
				if judgmentKey(round, item) == meta.TargetKey {
					return e.judgmentBriefText(ctx, state, p, round, item, &meta)
				}
			}
		}
	}
	return brief.Brief{}, errors.New("pre-judgment instruction round is missing")
}

func (e *Engine) acceptJudgmentBrief(instruction ledger.Instruction, obs observedResult) (bool, error) {
	var result struct {
		InstructionID string   `json:"instruction_id"`
		Instructions  []string `json:"instructions"`
	}
	if err := e.forms["judgment-brief"].Decode(obs.result.Content, &result); err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	if result.InstructionID != instruction.ID {
		return e.rejectResult(instruction, obs, errors.New("pre-judgment instruction ID does not match"))
	}
	rest := strings.TrimPrefix(instruction.TargetKey, instruction.RoundID+":")
	cut := strings.LastIndex(rest, ":")
	if cut < 0 {
		return false, errors.New("pre-judgment instruction key is malformed")
	}
	_, err := e.append(&ledger.JudgmentBrief{InstructionID: instruction.ID, RoundID: instruction.RoundID, Aspect: rest[:cut], Runner: rest[cut+1:], Instructions: result.Instructions, AgentID: obs.result.AgentID, ResultPath: obs.path})
	return true, err
}
