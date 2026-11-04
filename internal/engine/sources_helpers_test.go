package engine

import "github.com/ildjenty/dev-kit/internal/plan"

// judgeCheck returns a judge-run check that selects the kit review document and examples of aspect.
func judgeCheck(aspect string, required bool) plan.Check {
	return plan.Check{
		Aspect: aspect, Runner: "devkit-judge", Required: required,
		Criteria: []plan.Selection{{Source: "principles", Select: "review/" + aspect + ".md"}},
		Examples: &plan.Selection{Source: "examples", Select: aspect + "/spec.json"},
	}
}
