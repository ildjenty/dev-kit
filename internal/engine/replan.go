package engine

import (
	"errors"

	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func (e *Engine) remainingPlan(state *ledger.State) (*plan.Plan, error) {
	run, ok := state.LatestStageRun()
	if !ok {
		return nil, nil
	}
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	for i := len(records) - 1; i >= 0; i-- {
		issued, ok := records[i].Payload.(*ledger.Instruction)
		if ok && records[i].ID > run.ID && issued.Origin == plan.BackProposal && issued.Context != nil && issued.Context.RemainingPlan != nil {
			return issued.Context.RemainingPlan, nil
		}
	}
	bound, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("bound plan is missing")
	}
	action := plan.Back
	if run.State == ledger.StageCompleted {
		action = plan.Done
	}
	next, err := bound.Next(run.StageID, action)
	if err != nil {
		return nil, err
	}
	// A replan edge terminates this segment; the bound plan orders its remaining segment.
	if next != nil && *next == "replan" {
		next = nil
		for i, stage := range bound.Stages {
			if stage.ID == run.StageID && i+1 < len(bound.Stages) {
				id := bound.Stages[i+1].ID
				next = &id
				break
			}
		}
	}
	remaining := bound
	remaining.Stages, remaining.StartStage = nil, ""
	if next != nil {
		remaining.StartStage = *next
	}
	seen := map[string]bool{}
	for next != nil && *next != "replan" && !seen[*next] {
		seen[*next] = true
		stage, ok := bound.Stage(*next)
		if !ok {
			return nil, errors.New("remaining stage is missing from the bound plan")
		}
		remaining.Stages = append(remaining.Stages, stage)
		next = stage.Transitions.Done
	}
	return &remaining, nil
}
