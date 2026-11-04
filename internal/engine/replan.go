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
	// A replan edge ends the bound plan's done path; the next plan supplies the rest.
	if next != nil && *next == "replan" {
		next = nil
	}
	if action == plan.Back && next != nil {
		bound, err = e.backPlan(state, run, *next, bound)
		if err != nil {
			return nil, err
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

// backPlan uses the return instance's adopted execution when it belongs to an older plan.
func (e *Engine) backPlan(state *ledger.State, run *ledger.StageRun, target string, bound plan.Plan) (plan.Plan, error) {
	if _, ok := bound.Stage(target); ok {
		return bound, nil
	}
	adopted, err := e.adoptedBefore(state, run.ID)
	if err != nil {
		return plan.Plan{}, err
	}
	for _, previous := range adopted {
		if previous.StageID == target {
			p, ok, err := e.boundPlan(previous.PlanVersion)
			if err != nil {
				return plan.Plan{}, err
			}
			if ok {
				return p, nil
			}
		}
	}
	return plan.Plan{}, errors.New("return instance is missing from the adopted execution path")
}
