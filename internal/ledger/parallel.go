package ledger

import (
	"fmt"
)

// UnitResults returns accepted results for one round, in acceptance order.
func (s *State) UnitResults(roundID string) []*UnitResult {
	var results []*UnitResult
	for _, record := range s.records {
		if result, ok := record.Payload.(*UnitResult); ok && result.RoundID == roundID {
			results = append(results, result)
		}
	}
	return results
}

// RoundMerge returns the latest merge checkpoint for one round.
func (s *State) RoundMerge(roundID string) (*Merge, bool) {
	for i := len(s.records) - 1; i >= 0; i-- {
		if merge, ok := s.records[i].Payload.(*Merge); ok && merge.RoundID == roundID {
			return merge, true
		}
	}
	return nil, false
}

func validateMerge(m *Merge) error {
	if err := required(m.RoundID, m.Head, m.Current, m.BeforeTree, m.Status); err != nil {
		return err
	}
	if len(m.Inputs) == 0 || m.Applied < 0 || m.Applied > len(m.Inputs) {
		return fmt.Errorf("invalid merge inputs or applied count")
	}
	for _, input := range m.Inputs {
		if err := required(input.InstructionID, input.UnitID, input.Commit); err != nil {
			return err
		}
	}
	switch m.Status {
	case "ready", "applying", "conflict":
		if m.Applied == len(m.Inputs) {
			return fmt.Errorf("unfinished merge requires a pending input")
		}
	case "merged":
		if m.Applied != len(m.Inputs) {
			return fmt.Errorf("finished merge requires all inputs")
		}
	default:
		return fmt.Errorf("unknown merge status: %q", m.Status)
	}
	if m.Status == "ready" {
		return nil
	}
	return required(m.ResultTree)
}

// JudgmentBriefs returns accepted pre-judgment instructions for a round.
func (s *State) JudgmentBriefs(roundID string) []*JudgmentBrief {
	briefs := []*JudgmentBrief{}
	for _, record := range s.records {
		if brief, ok := record.Payload.(*JudgmentBrief); ok && brief.RoundID == roundID {
			briefs = append(briefs, brief)
		}
	}
	return briefs
}
