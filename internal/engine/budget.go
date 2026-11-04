package engine

import (
	"fmt"
	"slices"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

const totalRoundLimit = "total_round_limit"

// totalRounds counts every round of the work. Backs, replans, and assignee changes never reduce it.
func totalRounds(state *ledger.State) []string {
	ids := []string{}
	for _, run := range state.StageRuns() {
		for _, round := range state.Rounds(run.ID) {
			ids = append(ids, round.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

// roundBudget asks the human before a round when the work-wide [limits] max_total_rounds is
// reached. Each approval to continue moves the next stop to the cumulative count at that question
// plus the limit. It returns a question to issue, or a message when continuation was declined.
func (e *Engine) roundBudget(state *ledger.State, settings config.Config, tree string) (*form.Question, string, error) {
	if settings.Limits.MaxTotalRounds == nil {
		return nil, "", nil
	}
	limit := *settings.Limits.MaxTotalRounds
	rounds := totalRounds(state)
	threshold := limit
	answers := state.AnsweredQuestions()
	var declined *form.Question
	for _, q := range state.Questions() {
		answer, ok := answers[q.ID]
		if q.StopCategory != totalRoundLimit || !ok {
			continue
		}
		if answer.Answer.Interpretation != ledger.AnswerApproved {
			copy := q
			declined = &copy
			continue
		}
		before := len(slices.DeleteFunc(slices.Clone(rounds), func(id string) bool { return id > q.ID }))
		threshold = max(threshold, before+limit)
	}
	if len(rounds) < threshold || len(rounds) == 0 {
		return nil, "", nil
	}
	evidence := []form.Reference{{RecordID: rounds[len(rounds)-1]}}
	if declined != nil && slices.Equal(declined.Evidence, evidence) {
		return nil, "The work-wide round limit was reached and continuation was not approved. Answer the question again to continue, or run devkit abort.", nil
	}
	text := fmt.Sprintf("The work has used %d rounds, reaching the work-wide limit ([limits] max_total_rounds = %d). Approve to continue for up to %d more rounds; reject to stop here.", len(rounds), limit, limit)
	q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: tree}, totalRoundLimit, text, "")
	q.Evidence = evidence
	saved, err := e.saveQuestion(state, q, "")
	return &saved, "", err
}
