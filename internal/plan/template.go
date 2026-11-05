package plan

import "slices"

// RequiresApproval compares with the last human-approved remaining plan.
// The caller supplies a validated policy and removes completed stages from lastApproved.
// The deviation policy requires approval for every change requirements does,
// plus any structural difference.
func (p Plan) RequiresApproval(policy string, lastApproved *Plan) bool {
	if lastApproved == nil || policy == "always" || len(p.OpenQuestions) > 0 || len(p.ExternalOperations) > 0 {
		return true
	}
	if !sameConditions(p.CompletionConditions, lastApproved.CompletionConditions) {
		return true
	}
	return policy == "deviation" && len(StructuralDifference(*lastApproved, p)) > 0
}

func sameConditions(conditions, approved []CompletionCondition) bool {
	if len(conditions) != len(approved) {
		return false
	}
	for _, condition := range conditions {
		if !slices.Contains(approved, condition) {
			return false
		}
	}
	return true
}
