package refine

import (
	"fmt"
	"os"
	"reflect"
	"slices"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func verifyChecks(ctx record.ContextData, artifacts string, draft doc, env cli.Env) (doc, error) {
	if !slices.ContainsFunc(items(draft["bindings"]), func(b doc) bool { return b["path"] == ".devkit/config.json" }) {
		return nil, nil
	}
	input, err := readJSON(file(artifacts, "input.json"))
	if err != nil {
		return nil, err
	}
	base := str(input["proposal_base"])
	paths, err := gitx.Run(ctx.Cwd, "ls-tree", "--name-only", base, "--", ":(literal).devkit/config.json")
	if err != nil {
		return nil, err
	}
	original := config.Defaults("")
	if paths != "" {
		data, err := gitx.Run(ctx.Cwd, "show", base+":.devkit/config.json")
		if err != nil {
			return nil, err
		}
		original, err = config.Parse([]byte(data))
		if err != nil {
			return nil, err
		}
	}
	current, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	selected := []config.Check{}
	for _, check := range current.Checks {
		if !slices.ContainsFunc(original.Checks, func(old config.Check) bool { return reflect.DeepEqual(old, check) }) {
			selected = append(selected, check)
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	last, err := readJSON(file(artifacts, "rounds/latest.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	round := num(last["round"]) + 1
	logs := file(artifacts, fmt.Sprintf("checks/%d", round))
	if err = os.MkdirAll(logs, 0755); err != nil {
		return nil, err
	}
	results, problems, err := checks.RunOnBase(ctx.Cwd, ctx.BaseBranch, selected, logs, env.Environ)
	if err != nil {
		return nil, err
	}
	report := doc{"base_checks": results, "errors": problems}
	if err = writeJSON(file(artifacts, fmt.Sprintf("verification/%d.json", round)), report); err != nil {
		return nil, err
	}
	return report, writeJSON(file(artifacts, "verification.json"), report)
}
