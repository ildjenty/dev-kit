package judge

import (
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func planOf(requirements, plan doc) doc {
	if requirements == nil || plan == nil || plan["requirements_version"] != requirements["version"] {
		return nil
	}
	return plan
}
func acceptedObjections(valid, judgments []doc) []doc {
	runs := map[string]bool{}
	for _, r := range valid {
		if r["workflow"] == "fix" {
			runs[text(r["run_id"])] = true
		}
	}
	out := []doc{}
	for _, j := range judgments {
		if j["judge"] != "dispute" || j["kind"] != "code" || !runs[text(j["run_id"])] {
			continue
		}
		for _, f := range docs(j["findings"]) {
			if !truth(f["blocking"]) {
				out = append(out, doc{"run_id": j["run_id"], "commit": object(j["target"])["commit"], "id": f["id"], "text": f["text"], "evidence": f["evidence"]})
			}
		}
	}
	return out
}
func validBefore(h record.HistoryData, runID string) ([]doc, error) {
	runs := []doc{}
	for _, r := range h.Runs {
		if r["run_id"] != runID {
			runs = append(runs, r)
		}
	}
	h.Runs = runs
	return record.ValidResults(h, "")
}
func prepare(ctx record.ContextData, artifacts, shownArtifacts string) (doc, error) {
	out := filepath.Join(artifacts, "judge")
	if err := os.MkdirAll(filepath.Join(out, "materials"), 0755); err != nil {
		return nil, err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	commit, err := g.Head()
	if err != nil {
		return nil, err
	}
	base, err := g.MergeBase(commit)
	if err != nil {
		return nil, err
	}
	stop := func(reason string) (doc, error) {
		return doc{"stopped": true, "reason": reason, "details": []string{}, "base": base, "commit": commit, "criteria_commit": "", "has_requirements": false, "has_plan": false, "accepted_objections_count": 0}, nil
	}
	clean, err := g.Clean()
	if err != nil {
		return nil, err
	}
	if !clean {
		return stop("the working tree has changes that are not committed")
	}
	if base == "" {
		return stop("HEAD has no merge base with the base branch")
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return stop("no request records the fixed criteria commit")
	}
	request := requests[0]
	source, err := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: text(request["criteria_commit"]), Dir: filepath.Join(artifacts, "criteria")})
	if err != nil {
		return nil, err
	}
	if len(source.Paths) == 0 {
		return nil, fmt.Errorf("criteria source has no standards path")
	}
	standards := source.Paths[0]
	mapping, err := concerns.Load(filepath.Join(source.Dir, standards))
	if err != nil {
		return nil, err
	}
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return nil, err
	}
	plan, err := record.Latest(ctx.Dir, "plan")
	if err != nil {
		return nil, err
	}
	plan = planOf(requirements, plan)
	for name, value := range map[string]doc{"requirements.json": requirements, "plan.json": plan} {
		if value != nil {
			if err = save(filepath.Join(out, name), value); err != nil {
				return nil, err
			}
		}
	}
	answers, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return nil, err
	}
	if err = save(filepath.Join(out, "answers.json"), doc{"answers": answers}); err != nil {
		return nil, err
	}
	h, err := record.History(ctx.Dir)
	if err != nil {
		return nil, err
	}
	valid, err := validBefore(h, ctx.RunID)
	if err != nil {
		return nil, err
	}
	judgments, err := record.List(ctx.Dir, "judgment")
	if err != nil {
		return nil, err
	}
	accepted := acceptedObjections(valid, judgments)
	if err = save(filepath.Join(out, "accepted-objections.json"), doc{"accepted": accepted}); err != nil {
		return nil, err
	}
	tree, err := judgedTree(ctx, g, commit)
	if err != nil {
		return nil, err
	}
	changes, err := g.Changes(base, commit)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(out, "diff.patch"), []byte(changes.Patch), 0644); err != nil {
		return nil, err
	}
	id, err := g.ChangeID(commit)
	if err != nil {
		return nil, err
	}
	target := doc{"base_branch": ctx.BaseBranch, "base": base, "commit": commit, "tree": tree, "change_id": id, "criteria_commit": request["criteria_commit"], "requirements_version": requirements["version"], "plan_version": plan["version"], "language": c.Language, "standards": standards}
	if err = save(filepath.Join(out, "target.json"), target); err != nil {
		return nil, err
	}
	allChecks, err := config.ChecksAcrossBase(ctx.Cwd, ctx.BaseBranch)
	if err != nil {
		return nil, err
	}
	checkConfig := c
	checkConfig.Checks = allChecks
	for _, q := range mapping.Judged("code") {
		selected := mapping.Materials(filepath.Join(shownArtifacts, "criteria", standards), q, "code")
		material := doc{"language": c.Language, "standards": standards, "quality": q, "criteria_folder": filepath.Join(shownArtifacts, "criteria", standards), "criteria": selected.Criteria, "principles": selected.Principles, "every_quality": selected.EveryQuality, "common": selected.Common, "authority": selected.Authority, "tree": tree, "checks": config.ChecksFor(checkConfig, changes), "changes": []doc{}, "rows": []doc{}, "without_plan": []doc{}}
		if err = save(filepath.Join(out, "materials", q+".json"), material); err != nil {
			return nil, err
		}
	}
	return doc{"stopped": false, "reason": "", "details": []string{}, "base": base, "commit": commit, "criteria_commit": request["criteria_commit"], "has_requirements": requirements != nil, "has_plan": plan != nil, "accepted_objections_count": len(accepted)}, nil
}
