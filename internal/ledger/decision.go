package ledger

import (
	"fmt"

	"github.com/ildjenty/dev-kit/internal/form"
)

// DecisionContext is empty because conditions and external references are passed through.
type DecisionContext struct{}

// ResolvedDecision preserves conditions alongside recorded status and graph relationships.
// Active describes those records, not whether natural-language conditions are satisfied.
type ResolvedDecision struct {
	RecordID             string           `json:"record_id"`
	Decision             form.Decision    `json:"decision"`
	Status               DecisionStatus   `json:"status"`
	Active               bool             `json:"active"`
	SupersededBy         []form.Reference `json:"superseded_by"`
	InactiveDependencies []form.Reference `json:"inactive_dependencies"`
}

func validateDecisionAnswer(record, referenced Record) error {
	change, ok := record.Payload.(*StateChange)
	if !ok || change.AnswerID != referenced.ID {
		return nil
	}
	var answer Answer
	switch p := referenced.Payload.(type) {
	case *Answer:
		answer = *p
	case *Approval:
		answer = p.Answer
	default:
		return fmt.Errorf("decision state change requires an answer or approval")
	}
	if answer.Target.Kind != form.ApprovalDecision || answer.Target.DecisionID != change.DecisionID {
		return fmt.Errorf("decision state change does not match answer target")
	}
	if answer.Interpretation == AnswerText || DecisionStatus(answer.Interpretation) != change.DecisionStatus {
		return fmt.Errorf("decision status %q does not match answer interpretation %q", change.DecisionStatus, answer.Interpretation)
	}
	return nil
}

// ResolveDecisions resolves status changes, active replacements, and dependencies.
// Conditions and external dependencies are preserved for agents and people to assess.
func (s *State) ResolveDecisions(_ DecisionContext) ([]ResolvedDecision, error) {
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
				SupersededBy: []form.Reference{}, InactiveDependencies: []form.Reference{},
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
	graph := decisionGraph{decisions: decisions, byRecord: byRecord, replacements: replacements,
		visited: make(map[int]bool), visiting: make(map[int]bool)}
	for index := range decisions {
		if _, err := graph.resolve(index); err != nil {
			return nil, err
		}
	}
	return decisions, nil
}

type decisionGraph struct {
	decisions    []ResolvedDecision
	byRecord     map[string]int
	replacements map[int][]int
	visited      map[int]bool
	visiting     map[int]bool
}

func (g decisionGraph) resolve(index int) (bool, error) {
	if g.visited[index] {
		return g.decisions[index].Active, nil
	}
	decision := &g.decisions[index]
	if g.visiting[index] {
		return false, fmt.Errorf("decision graph contains a cycle at %s", decision.RecordID)
	}
	g.visiting[index] = true
	valid := decision.Status != DecisionRejected && decision.Status != DecisionDeferred
	for _, ref := range decision.Decision.DependsOn {
		var active bool
		if ref.RecordID != "" {
			dependency, exists := g.byRecord[ref.RecordID]
			if !exists {
				return false, fmt.Errorf("decision %s: dependency %q is not a decision", decision.RecordID, ref.RecordID)
			}
			var err error
			active, err = g.resolve(dependency)
			if err != nil {
				return false, err
			}
		} else {
			continue
		}
		if !active {
			decision.InactiveDependencies = append(decision.InactiveDependencies, ref)
			valid = false
		}
	}
	for _, replacement := range g.replacements[index] {
		active, err := g.resolve(replacement)
		if err != nil {
			return false, err
		}
		if active || len(g.decisions[replacement].SupersededBy) != 0 {
			decision.SupersededBy = append(decision.SupersededBy, form.Reference{RecordID: g.decisions[replacement].RecordID})
			valid = false
		}
	}
	decision.Active = valid
	g.visiting[index], g.visited[index] = false, true
	return valid, nil
}
