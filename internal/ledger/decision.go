package ledger

import (
	"fmt"

	"github.com/ildjenty/dev-kit/internal/form"
)

// DecisionContext supplies assessments of free-form validity and application conditions.
// ExternalValid assesses explicitly external dependencies, which are outside the work graph.
type DecisionContext struct {
	Valid         func(form.Decision) (bool, error)
	ExternalValid func(form.Reference) (bool, error)
}

type ResolvedDecision struct {
	RecordID            string           `json:"record_id"`
	Decision            form.Decision    `json:"decision"`
	Status              DecisionStatus   `json:"status"`
	Valid               bool             `json:"valid"`
	SupersededBy        []form.Reference `json:"superseded_by"`
	InvalidDependencies []form.Reference `json:"invalid_dependencies"`
}

// ResolveDecisions resolves status changes, active replacements, and dependencies.
// It does not infer the meaning of free-form validity or conditions.
func (s *State) ResolveDecisions(context DecisionContext) ([]ResolvedDecision, error) {
	decisions := []ResolvedDecision{}
	byRecord := make(map[string]int)
	byID := make(map[string]int)
	for _, record := range s.records {
		if decision, ok := record.Payload.(*Decision); ok {
			if _, duplicate := byID[decision.ID]; duplicate {
				return nil, fmt.Errorf("duplicate decision ID: %q", decision.ID)
			}
			byRecord[record.ID], byID[decision.ID] = len(decisions), len(decisions)
			decisions = append(decisions, ResolvedDecision{
				RecordID: record.ID, Decision: decision.Decision, Status: DecisionPending,
				SupersededBy: []form.Reference{}, InvalidDependencies: []form.Reference{},
			})
		}
	}
	answers := make(map[string]Answer)
	for _, answer := range s.AnsweredQuestions() {
		answers[answer.RecordID] = answer.Answer
	}
	for _, record := range s.records {
		change, ok := record.Payload.(*StateChange)
		if !ok || change.DecisionID == "" {
			continue
		}
		index, exists := byID[change.DecisionID]
		if !exists {
			return nil, fmt.Errorf("decision state change %s: decision %q not found", record.ID, change.DecisionID)
		}
		answer, effective := answers[change.AnswerID]
		if effective && answer.Target.Kind == form.ApprovalDecision && answer.Target.DecisionID == change.DecisionID {
			decisions[index].Status = change.DecisionStatus
		}
	}
	replacements := make(map[int][]int)
	for index, decision := range decisions {
		for _, ref := range decision.Decision.Supersedes {
			if ref.RecordID == "" {
				continue
			}
			old, exists := byRecord[ref.RecordID]
			if !exists {
				return nil, fmt.Errorf("decision %s: replacement target %q is not a decision", decision.RecordID, ref.RecordID)
			}
			replacements[old] = append(replacements[old], index)
		}
	}
	visited, visiting := make(map[int]bool), make(map[int]bool)
	var resolve func(int) (bool, error)
	resolve = func(index int) (bool, error) {
		if visited[index] {
			return decisions[index].Valid, nil
		}
		decision := &decisions[index]
		if visiting[index] {
			return false, fmt.Errorf("decision graph contains a cycle at %s", decision.RecordID)
		}
		visiting[index] = true
		valid := decision.Status != DecisionRejected && decision.Status != DecisionDeferred
		if context.Valid != nil {
			assessed, err := context.Valid(decision.Decision)
			if err != nil {
				return false, fmt.Errorf("decision %s: %w", decision.RecordID, err)
			}
			valid = valid && assessed
		} else if decision.Decision.Validity != "" || len(decision.Decision.Conditions) != 0 {
			return false, fmt.Errorf("decision %s: validity assessment is required", decision.RecordID)
		}
		for _, ref := range decision.Decision.DependsOn {
			var active bool
			if ref.RecordID != "" {
				dependency, exists := byRecord[ref.RecordID]
				if !exists {
					return false, fmt.Errorf("decision %s: dependency %q is not a decision", decision.RecordID, ref.RecordID)
				}
				var err error
				active, err = resolve(dependency)
				if err != nil {
					return false, err
				}
			} else {
				if context.ExternalValid == nil {
					return false, fmt.Errorf("decision %s: external dependency assessment is required", decision.RecordID)
				}
				var err error
				active, err = context.ExternalValid(ref)
				if err != nil {
					return false, fmt.Errorf("decision %s: %w", decision.RecordID, err)
				}
			}
			if !active {
				decision.InvalidDependencies = append(decision.InvalidDependencies, ref)
				valid = false
			}
		}
		for _, replacement := range replacements[index] {
			active, err := resolve(replacement)
			if err != nil {
				return false, err
			}
			if active || len(decisions[replacement].SupersededBy) != 0 {
				decision.SupersededBy = append(decision.SupersededBy, form.Reference{RecordID: decisions[replacement].RecordID})
				valid = false
			}
		}
		decision.Valid = valid
		visiting[index], visited[index] = false, true
		return valid, nil
	}
	for index := range decisions {
		if _, err := resolve(index); err != nil {
			return nil, err
		}
	}
	return decisions, nil
}
