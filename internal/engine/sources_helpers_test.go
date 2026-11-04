package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// withCatalog appends the installed check catalog and planCatalog to a test configuration.
func withCatalog(settings string) string {
	fragment, err := kit.HostFragment("roles.toml")
	if err != nil {
		panic(err)
	}
	catalog := string(fragment)
	catalog = catalog[strings.Index(catalog, "[checks."):strings.Index(catalog, "[[loop.")] + planCatalog
	kept := []string{}
	for _, block := range strings.SplitAfter(catalog, "\n\n") {
		header, _, _ := strings.Cut(strings.TrimSpace(block), "\n")
		if !strings.Contains(settings, header) && !strings.Contains(settings, strings.ReplaceAll(header, `"`, "")) {
			kept = append(kept, block)
		}
	}
	return settings + "\n" + strings.Join(kept, "")
}

// judgeEntry configures a judge-run check with the kit review document and examples of aspect
// and extra settings such as history or brief_by_planner.
func judgeEntry(aspect, extra string) string {
	return "\n[checks." + aspect + "]\nrunner = 'judge'\ncriteria = [{ source = 'principles', select = 'review/" + aspect + ".md' }]\nexamples = { source = 'examples', select = '" + aspect + "/spec.json' }\n" + extra + "\n"
}

// planCatalog configures the plan review as a stage check and the test command checks.
const planCatalog = "\n\n[checks.plan]\nrunner = 'judge'\ncriteria = [{ source = 'principles', select = 'review/plan.md' }, { source = 'plans', select = 'README.md' }]\nexamples = { source = 'examples', select = 'plan/spec.json' }\n\n[checks.\"command:fail\"]\nrunner = 'command:fail'\n\n[checks.\"command:verify\"]\nrunner = 'command:verify'\n"

// judgeCheck returns a plan check that selects the configured judge-run check of aspect.
func judgeCheck(aspect string, required bool) plan.Check {
	return plan.Check{Aspect: aspect, Required: required}
}

// catalogJudge returns judgeCheck resolved with the kit review document and examples of aspect.
func catalogJudge(aspect string, required bool) catalogCheck {
	item := catalogCheck{judgeCheck(aspect, required), plan.CheckDefinition{
		Runner:   "judge",
		Criteria: []plan.Selection{{Source: "principles", Select: "review/" + aspect + ".md"}},
		Examples: &plan.Selection{Source: "examples", Select: aspect + "/spec.json"},
	}}
	if aspect == "plan" {
		item.Criteria = planSelections((config.Config{}).PlanCheckSettings().Criteria)
	}
	return item
}

// configureCatalogEntry replaces the committed catalog entry of aspect with judgeEntry(aspect, extra).
func configureCatalogEntry(t *testing.T, e *Engine, aspect, extra string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	settings := string(data)
	for _, header := range []string{"[checks.\"" + aspect + "\"]", "[checks." + aspect + "]"} {
		if start := strings.Index(settings, header); start >= 0 {
			end := strings.Index(settings[start+len(header):], "\n[")
			if end < 0 {
				settings = settings[:start]
			} else {
				settings = settings[:start] + settings[start+len(header)+end+1:]
			}
		}
	}
	write(t, e.Root(), "devkit.toml", settings+judgeEntry(aspect, extra))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure check "+aspect)
}
