package ledger

import (
	"testing"

	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestMetricsCountInstancesAndDefinitions(t *testing.T) {
	store, _ := workStore(t)
	stage := func(id string) plan.Stage {
		return plan.Stage{ID: id, Definition: "implement", Transitions: plan.Transitions{Fix: id, Ask: id, Back: id}}
	}
	first, second := stage("implement-1"), stage("implement-2")
	done := "implement-2"
	first.Transitions.Done = &done
	appendRecord(t, store, &PlanVersion{Plan: plan.Plan{Version: 1, Pattern: plan.PatternRef{Name: "default", ContentHash: "hash"}, Task: "Build",
		StartStage: first.ID, Stages: []plan.Stage{first, second}, ExternalOperations: []plan.ExternalOperation{}}, Origin: plan.InitialProposal})
	for _, id := range []string{"implement-1", "implement-2"} {
		run := &StageRun{StageID: id, PlanVersion: 1, State: StageWorking}
		appendRecord(t, store, run)
		appendRecord(t, store, &Round{StageRunID: run.ID, SessionID: "session"})
	}
	m := loadState(t, store).Metrics()
	if len(m.Stages) != 2 || m.Stages[0].StageID != "implement-1" || m.Stages[0].Rounds != 1 {
		t.Fatalf("instance counts: %+v", m.Stages)
	}
	if len(m.Definitions) != 1 || m.Definitions[0] != (DefinitionMetrics{Definition: "implement", Instances: 2, Runs: 2, Rounds: 2}) {
		t.Fatalf("definition counts: %+v", m.Definitions)
	}
}
