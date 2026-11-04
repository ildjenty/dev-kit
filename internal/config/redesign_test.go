package config

import (
	"reflect"
	"testing"
)

func TestPlanCheckSettings(t *testing.T) {
	for _, tc := range []struct {
		body string
		want PlanCheck
	}{
		{"", PlanCheck{Aspect: "plan", Runner: "judge"}},
		{"[plan_check]\naspect = 'custom-plan'", PlanCheck{Aspect: "custom-plan", Runner: "judge"}},
		{"[plan_check]\nrunner = 'registered-judge'\n[judges.registered-judge]\nexecutable = 'judge'\ntimeout_seconds = 10\noutput_schema = 'schema.json'", PlanCheck{Aspect: "plan", Runner: "registered-judge"}},
		{"[plan_check]\naspect = 'custom-plan'\nrunner = 'custom-agent'\n[roles.custom-agent]\nkind = 'judge'\nagent = 'devkit-custom'", PlanCheck{Aspect: "custom-plan", Runner: "custom-agent"}},
	} {
		t.Run(tc.body, func(t *testing.T) {
			settings, err := Parse([]byte("kit_version = 'v3'\n" + tc.body))
			if err != nil || settings.PlanCheck.Aspect != tc.want.Aspect || settings.PlanCheck.Runner != tc.want.Runner {
				t.Fatalf("plan check settings = %+v, error = %v; want %+v", settings.PlanCheck, err, tc.want)
			}
		})
	}
	if got := (Config{}).PlanCheckSettings(); !reflect.DeepEqual(got, PlanCheck{Inputs: effectiveInputs(nil), Aspect: "plan", Runner: "judge", Criteria: []Selection{{Source: "principles", Select: "review/plan.md"}, {Source: "plans", Select: "README.md"}}, Examples: &Selection{Source: "examples", Select: "plan/spec.json"}, Schemas: []string{"plan", "plan-template", "finding"}}) {
		t.Fatalf("effective plan check defaults = %+v", got)
	}
}

func TestRouterConfirmationDoesNotControlLoopEdits(t *testing.T) {
	for _, policy := range []string{"confirm", "allow"} {
		settings, err := Parse([]byte("kit_version = 'v3'\n[router]\nhuman_confirmation = []\n[approval]\nmeta_loop_edit = '" + policy + "'"))
		if err != nil || len(settings.HumanConfirmation()) != 0 || settings.MetaLoopEditApproval() != policy {
			t.Fatalf("loop edit policy = %q, error = %v; want %q", settings.MetaLoopEditApproval(), err, policy)
		}
	}
}
