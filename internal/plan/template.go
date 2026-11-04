package plan

import "slices"

// RequiresApproval compares with the last human-approved remaining plan.
// The caller supplies a validated policy and removes completed stages from lastApproved.
func (p Plan) RequiresApproval(policy string, lastApproved *Plan) bool {
	if lastApproved == nil || policy == "always" || len(p.OpenQuestions) > 0 || len(p.ExternalOperations) > 0 {
		return true
	}
	if policy == "deviation" {
		return len(StructuralDifference(*lastApproved, p)) > 0
	}
	if len(p.CompletionConditions) != len(lastApproved.CompletionConditions) {
		return true
	}
	for _, condition := range p.CompletionConditions {
		if !slices.Contains(lastApproved.CompletionConditions, condition) {
			return true
		}
	}
	return false
}
