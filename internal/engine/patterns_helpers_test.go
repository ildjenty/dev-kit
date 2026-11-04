package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// newPlan copies a pattern's default stages into a first plan that cites the pattern.
func newPlan(pattern plan.PlanTemplate, contentHash, task string, conditions []plan.CompletionCondition) plan.Plan {
	p := plan.Plan{Version: 1, Pattern: plan.PatternRef{Name: pattern.Name, ContentHash: contentHash}, Task: task,
		CompletionConditions: conditions, StartStage: pattern.StartStage, ExternalOperations: []plan.ExternalOperation{}}
	data, _ := json.Marshal(pattern.Stages)
	_ = json.Unmarshal(data, &p.Stages)
	return p
}

// withoutLoopRules removes the installed [[loop.stage]] and [[loop.path]] rules so that test
// patterns with arbitrary stages are accepted; tests of the rules configure them explicitly.
func withoutLoopRules(settings string) string {
	values := map[string]any{}
	if err := toml.Unmarshal([]byte(settings), &values); err != nil {
		panic(err)
	}
	delete(values, "loop")
	var text strings.Builder
	if err := toml.NewEncoder(&text).Encode(values); err != nil {
		panic(err)
	}
	return text.String()
}

// chainStages keeps the named stages of p in the given order and links their done transitions
// in that order; the last stage keeps its own done transition.
func chainStages(p plan.Plan, ids ...string) plan.Plan {
	result := p
	result.Stages = nil
	for i, id := range ids {
		stage, _ := p.Stage(id)
		if i+1 < len(ids) {
			next := ids[i+1]
			stage.Transitions.Done = &next
		}
		result.Stages = append(result.Stages, stage)
	}
	result.StartStage = ids[0]
	return result
}

// approvedNext approves a pending plan approval question, which every first plan asks, and
// returns the next result; other results are returned unchanged.
func approvedNext(t *testing.T, e *Engine, result Result) Result {
	t.Helper()
	for result.Instruction != nil && result.Instruction.Question != nil && result.Instruction.Question.StopCategory == "plan_approval" {
		observeAnswer(t, e, "Approve this plan.")
		if _, err := e.Answer(t.Context(), AnswerRequest{QuestionID: result.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
			t.Fatal(err)
		}
		next, err := e.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		result = next
	}
	return result
}
