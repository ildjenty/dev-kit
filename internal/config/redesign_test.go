package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestPlanCheckSettings(t *testing.T) {
	for _, tc := range []struct {
		body string
		want PlanCheck
	}{
		{"", PlanCheck{Aspect: "plan", Runner: "devkit-judge"}},
		{"[plan_check]\naspect = 'custom-plan'", PlanCheck{Aspect: "custom-plan", Runner: "devkit-judge"}},
		{"[plan_check]\nrunner = 'registered-judge'", PlanCheck{Aspect: "plan", Runner: "registered-judge"}},
		{"[plan_check]\naspect = 'custom-plan'\nrunner = 'custom-agent'", PlanCheck{Aspect: "custom-plan", Runner: "custom-agent"}},
	} {
		t.Run(tc.body, func(t *testing.T) {
			settings, err := Parse([]byte("kit_version = 'v2'\n" + tc.body))
			if err != nil || settings.PlanCheck != tc.want || settings.PlanCheckSettings() != tc.want {
				t.Fatalf("plan check settings = %+v, error = %v; want %+v", settings.PlanCheck, err, tc.want)
			}
		})
	}
	if got := (Config{}).PlanCheckSettings(); got != (PlanCheck{Aspect: "plan", Runner: "devkit-judge"}) {
		t.Fatalf("effective plan check defaults = %+v", got)
	}
}

func TestRouterConfirmationDoesNotControlLoopEdits(t *testing.T) {
	for _, policy := range []string{"confirm", "allow"} {
		settings, err := Parse([]byte("kit_version = 'v2'\n[router]\nhuman_confirmation = []\n[approval]\nmeta_loop_edit = '" + policy + "'"))
		if err != nil || len(settings.HumanConfirmation()) != 0 || settings.MetaLoopEditApproval() != policy {
			t.Fatalf("loop edit policy = %q, error = %v; want %q", settings.MetaLoopEditApproval(), err, policy)
		}
	}
}

func TestRejectOverlappingRepositoryPaths(t *testing.T) {
	norms := []string{"principles", "stages", "checks", "plans", "templates", "rules"}
	for i, norm := range norms {
		for _, other := range norms[i+1:] {
			for _, pair := range [][2]string{{"norms", "norms"}, {"norms", "norms/child"}, {"norms/child", "norms"}} {
				t.Run(norm+"/"+other+"/"+pair[0]+"/"+pair[1], func(t *testing.T) {
					body := fmt.Sprintf("kit_version = 'v2'\n[paths]\n%s = %q\n%s = %q", norm, pair[0], other, pair[1])
					_, err := Parse([]byte(body))
					if err == nil || !strings.Contains(err.Error(), "paths."+norm) || !strings.Contains(err.Error(), "paths."+other) {
						t.Fatalf("overlapping norm paths: %v", err)
					}
				})
			}
		}
		for _, product := range []string{"requirements", "structure", "detect"} {
			for _, value := range []string{"norms", "norms/product"} {
				t.Run(norm+"/"+product+"/"+value, func(t *testing.T) {
					body := fmt.Sprintf("kit_version = 'v2'\n[paths]\n%s = 'norms'\n%s = %q", norm, product, value)
					_, err := Parse([]byte(body))
					if err == nil || !strings.Contains(err.Error(), "paths."+norm) || !strings.Contains(err.Error(), "paths."+product) {
						t.Fatalf("product path inside a norm: %v", err)
					}
				})
			}
		}
	}
	for _, body := range []string{
		"principles = 'norms'\nstages = 'norms-extra'\nrequirements = 'norms-product'",
		"principles = 'shared/principles'\nrequirements = 'shared/requirements.md'",
		"principles = 'products/norms'\nrequirements = 'products'",
	} {
		if _, err := Parse([]byte("kit_version = 'v2'\n[paths]\n" + body)); err != nil {
			t.Errorf("non-overlapping paths were rejected: %v", err)
		}
	}
}
