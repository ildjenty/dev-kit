package config

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/assets"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestDynamicDefaultsAndInputs(t *testing.T) {
	c, err := Parse([]byte("kit_version = 'v3'"))
	if err != nil {
		t.Fatal(err)
	}
	if c.PlanningMaxRounds() != 3 || !ceilingsEqual(c, 3, 2) || *c.Limits.MaxTotalRounds != 40 {
		t.Fatalf("incorrect dynamic defaults: %+v", c)
	}
	if !slices.Equal(c.PlanningInputs(), effectiveInputs(nil)) || !slices.Equal(c.PlanCheckSettings().Inputs, effectiveInputs(nil)) {
		t.Fatal("missing default inputs")
	}
	c, err = Parse([]byte(`kit_version = 'v3'
[planning]
max_rounds = 5
inputs = []
[plan_check]
inputs = [{source = 'rules', select = ''}]
[limits]
max_rounds_ceiling = 7
max_no_progress_ceiling = 4
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.PlanningMaxRounds() != 5 || len(c.PlanningInputs()) != 0 || c.PlanningInputs() == nil || !ceilingsEqual(c, 7, 4) {
		t.Fatal("dynamic overrides lost")
	}
	inputs := c.PlanCheckSettings().Inputs
	inputs[0].Source = "changed"
	if c.PlanCheck.Inputs[0].Source != "rules" {
		t.Fatal("accessor changed stored inputs")
	}
}

func TestBundledCheckRegistryAndRules(t *testing.T) {
	body, err := fs.ReadFile(assets.FS(), "hosts/roles.toml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(append([]byte("kit_version = 'v3'\n"), body...))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Loop.Stage) != 4 || len(c.Loop.Path) != 2 {
		t.Fatalf("missing bundled rules: %+v", c.Loop)
	}
	templateSchema, err := fs.ReadFile(assets.FS(), plan.TemplateSchemaPath)
	if err != nil {
		t.Fatal(err)
	}
	planSchema, err := fs.ReadFile(assets.FS(), plan.PlanSchemaPath)
	if err != nil {
		t.Fatal(err)
	}
	v, err := plan.NewValidator(templateSchema, planSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"default", "fix", "docs"} {
		body, err := fs.ReadFile(assets.FS(), "plans/"+name+".json")
		if err != nil {
			t.Fatal(err)
		}
		pattern, err := v.ParseTemplate(body)
		if err != nil {
			t.Fatal(err)
		}
		p := plan.Plan{StartStage: pattern.StartStage, Stages: pattern.Stages}
		for _, rule := range c.Loop.Stage {
			for _, stage := range p.Stages {
				if slices.Contains(rule.Definition, stage.DefinitionID()) {
					for _, aspect := range rule.Checks {
						if !slices.ContainsFunc(stage.Checks, func(check plan.Check) bool { return check.Aspect == aspect && check.Required }) {
							t.Fatalf("%s: missing required check %s", name, aspect)
						}
					}
					if rule.Gate && !stage.Gate.Required {
						t.Fatalf("%s/%s: missing the fixed human gate", name, stage.ID)
					}
					if rule.History != "" && string(stage.WorkerHistory()) != rule.History {
						t.Fatalf("%s/%s: history %q differs from the fixed %q", name, stage.ID, stage.WorkerHistory(), rule.History)
					}
				}
			}
		}
		for _, stage := range append(slices.Clone(pattern.Stages), pattern.OptionalStages...) {
			for _, check := range stage.Checks {
				definition, ok := c.Checks[check.Aspect]
				if !ok || definition.Runner == "" {
					t.Fatalf("missing configured check: %s", check.Aspect)
				}
				if slices.Contains([]string{"harness", "irreversible", "structure", "contract", "diff"}, check.Aspect) && definition.History != "all" {
					t.Fatalf("%s lacks full history", check.Aspect)
				}
			}
		}
		if name == "docs" && p.Stages[0].DefinitionID() != "edit" {
			t.Fatal("docs pattern uses implement definition")
		}
	}
}

func TestDynamicRuleShapes(t *testing.T) {
	c, err := Parse([]byte(`kit_version = 'v3'
[checks.test]
runner = 'judge'
criteria = [{source = 'rules', select = ''}]
[[loop.stage]]
definition = ['implement', 'edit']
checks = ['test']
gate = true
roles = ['worker', 'human']
max_rounds = 2
max_no_progress = 1
report = {source = 'templates', select = 'reports/implement.json'}
history = 'all'
outputs = []
[[loop.stage]]
definition = '*'
[[loop.path]]
require = ['purpose', 'retrospective']
[[loop.path]]
first = 'purpose'
`))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(c.Loop.Stage[0].Definition, Definitions{"implement", "edit"}) || c.Loop.Stage[0].Outputs == nil || len(*c.Loop.Stage[0].Outputs) != 0 || c.Loop.Stage[1].Definition[0] != "*" {
		t.Fatal("incorrect stage rule shape")
	}
}

func TestInvalidDynamicSettings(t *testing.T) {
	for _, body := range []string{
		"[planning]\nmax_rounds = 0",
		"[planning]\npatterns = ['default']",
		"[planning]\ninputs = [{source = 'missing', select = ''}]",
		"[plan_check]\ninputs = [{source = 'missing', select = ''}]",
		"[limits]\nmax_rounds_ceiling = 0",
		"[limits]\nmax_no_progress_ceiling = -1",
		"[checks.test]\nrunner = 'missing'",
		"[checks.test]\nrunner = 'worker'",
		"[checks.test]\nrunner = 'command:test'",
		"[checks.'command:test']\nrunner = 'judge'",
		"[checks.'command:']\nrunner = 'command:'",
		"[checks.test]\nrunner = 'judge'\nhistory = 'other'",
		"[checks.test]\nrunner = 'judge'\nschemas = ['../plan']",
		"[checks.test]\nrunner = 'judge'\ncriteria = [{source = 'requirements', select = ''}]",
		"[checks.'command:test']\nrunner = 'command:test'\ncriteria = [{source = 'rules', select = ''}]",
		"[[loop.rules]]\ntype = 'gate'",
		"[[loop.stage]]\ndefinition = []",
		"[[loop.stage]]\ndefinition = ['*', 'edit']",
		"[[loop.stage]]\ndefinition = [1]",
		"[[loop.stage]]\ndefinition = 'edit'\nchecks = ['unknown']",
		"[[loop.stage]]\ndefinition = 'edit'\nroles = ['judge']",
		"[[loop.stage]]\ndefinition = 'edit'\nmax_rounds = -1",
		"[[loop.stage]]\ndefinition = 'edit'\nmax_rounds = 0",
		"[[loop.stage]]\ndefinition = 'edit'\nhistory = 'other'",
		"[[loop.stage]]\ndefinition = 'edit'\nunknown = true",
		"[[loop.path]]\nfirst = 'edit'\nlast = 'retrospective'",
		"[[loop.path]]\nrequire = []",
		"[[loop.path]]\ndefinition = 'implement'",
		"[[loop.path]]\nfollows = 'verification'",
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := Parse([]byte("kit_version = 'v3'\n" + body)); err == nil {
				t.Fatal("accepted invalid dynamic settings")
			}
		})
	}
}

func ceilingsEqual(c Config, rounds, noProgress int) bool {
	a, b := c.StageLimitCeilings()
	return a == rounds && b == noProgress
}
