package config

import (
	"reflect"
	"testing"
)

func TestFlowSettings(t *testing.T) {
	defaults, err := Parse([]byte("kit_version = 'v3'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if defaults.BriefMaxBytes() != 200000 || defaults.Limits.MaxTotalRounds != nil || defaults.RoleMaxConcurrent("judge") != 3 || !reflect.DeepEqual(defaults.PlanCheckSettings().Schemas, []string{"plan", "plan-template"}) {
		t.Fatalf("unexpected flow defaults: %+v", defaults)
	}
	settings, err := Parse([]byte(`kit_version = 'v3'
[parallel]
max_concurrent = 4
[roles.judge]
max_concurrent = 2
[roles.specialist]
kind = 'work'
agent = 'devkit-specialist'
max_concurrent = 6
[briefs]
max_bytes = 512
[limits]
max_total_rounds = 10
[plan_check]
schemas = []
`))
	if err != nil {
		t.Fatal(err)
	}
	if settings.MaxConcurrent() != 4 || settings.RoleMaxConcurrent("judge") != 2 || settings.RoleMaxConcurrent("specialist") != 6 || settings.RoleMaxConcurrent("planner") != 4 || settings.BriefMaxBytes() != 512 || settings.Limits.MaxTotalRounds == nil || *settings.Limits.MaxTotalRounds != 10 || len(settings.PlanCheckSettings().Schemas) != 0 {
		t.Fatalf("unexpected flow settings: %+v", settings)
	}
	if settings.RoleSettings()["judge"].Agent != "devkit-judge" {
		t.Fatal("concurrency override lost the built-in role definition")
	}
}

func TestInvalidFlowSettings(t *testing.T) {
	for _, body := range []string{
		"[parallel]\nmax_workers = 2",
		"[briefs]\nmax_bytes = 0",
		"[briefs]\nmax_bytes = -1",
		"[limits]\nmax_total_rounds = 0",
		"[limits]\nmax_total_rounds = -1",
		"[roles.judge]\nmax_concurrent = 0",
		"[roles.judge]\nmax_concurrent = -1",
		"[roles.judge]\nmax_concurrent = '2'",
		"[roles.judge]\nmax_concurrent = 1.5",
		"[plan_check]\nschemas = ['../plan']",
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := Parse([]byte("kit_version = 'v3'\n" + body)); err == nil {
				t.Fatal("accepted invalid flow settings")
			}
		})
	}
}
