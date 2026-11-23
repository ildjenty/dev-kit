package pr

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func inputsOf(requirements, plan doc, valid []doc, accepted bool) doc {
	reasons := []string{}
	used := plan
	if plan == nil || requirements == nil || plan["requirements_version"] != requirements["version"] {
		used = nil
	}
	if requirements == nil {
		reasons = append(reasons, "no requirements are recorded")
	} else {
		var r doc
		for _, v := range valid {
			if v["workflow"] == "requirements" {
				r = v
			}
		}
		if r["outcome"] != "pass" || object(r["inputs"])["requirements_version"] != requirements["version"] {
			reasons = append(reasons, fmt.Sprintf("requirements %v have no passing result", requirements["version"]))
		}
	}
	if used != nil {
		if !accepted {
			reasons = append(reasons, fmt.Sprintf("plan %v has no passing or carried result", used["version"]))
		}
	} else {
		reasons = append(reasons, "no passing plan is recorded")
	}

	return doc{"reasons": reasons, "plan": used, "versions": doc{"requirements_version": requirements["version"], "plan_version": used["version"]}}
}
func asks(items, allowed []doc, rows ...doc) []doc {
	out := []doc{}
	seen := map[string]bool{}
	for _, i := range items {
		covered := false
		for _, row := range rows {
			if row["decided_by"] != "owner" {
				continue
			}
			for _, touch := range stringsOf(row["touches"]) {
				covered = covered || config.Covers(touch, text(i["file"])) || text(i["oldPath"]) != "" && config.Covers(touch, text(i["oldPath"]))
			}
		}
		if !truth(i["ask"]) || covered {
			continue
		}
		b, _ := json.Marshal([]any{i["category"], i["file"], i["item"]})
		key := string(b)
		if seen[key] {
			continue
		}
		seen[key] = true
		item := clone(i)
		item["allowed_by"] = nil
		for _, a := range allowed {
			if a["category"] == i["category"] && a["file"] == i["file"] && a["item"] == i["item"] && a["value"] == i["item"] {
				item["allowed_by"] = a["question_id"]
				break
			}
		}
		out = append(out, item)
	}
	return out
}
func itemsOf(g *gitx.Git, base, head string, c config.Config, changes gitx.Changes) ([]doc, error) {
	items := []doc{}
	for _, change := range config.DecisionChanges(c, changes) {
		args := []string{"diff", "--no-ext-diff", "--no-textconv", base, head, "--", ":(literal)" + change.Path}
		if change.OldPath != "" {
			args = append(args, ":(literal)"+change.OldPath)
		}
		patch, err := g.Run(args...)
		if err != nil {
			return nil, err
		}
		identity := struct {
			Status  string `json:"status"`
			OldPath string `json:"oldPath,omitempty"`
			Patch   string `json:"patch"`
		}{change.Status, change.OldPath, fmt.Sprintf("%x", sha256.Sum256([]byte(patch)))}
		identityJSON, err := jsonString(identity)
		if err != nil {
			return nil, err
		}
		itemBytes, err := json.Marshal(change)
		if err != nil {
			return nil, err
		}
		var item doc
		if err = json.Unmarshal(itemBytes, &item); err != nil {
			return nil, err
		}
		item["category"] = change.Quality
		item["file"] = change.Path
		item["item"] = identityJSON
		items = append(items, item)
	}
	return items, nil
}
func question(i doc) doc {
	ref := struct {
		Category any `json:"category"`
		File     any `json:"file"`
		Item     any `json:"item"`
	}{i["category"], i["file"], i["item"]}
	refJSON, _ := jsonString(ref)
	return doc{"text": fmt.Sprintf("Allow %s %s (%s)? Begin the answer with allow or deny.", text(i["status"]), text(i["file"]), text(i["category"])), "target": doc{"kind": "change", "ref": refJSON}, "next_workflows": []string{"pr"}, "invalidates": []int{}}
}
func checkpoints(requirements, plan doc, answers, items, judgments, carried []doc) doc {
	changed, checked, jj, remaining, notChecked, decisions := []doc{}, []doc{}, []doc{}, []doc{}, []doc{}, []doc{}
	for _, i := range items {
		changed = append(changed, doc{"status": i["status"], "path": i["path"], "oldPath": i["oldPath"], "quality": i["quality"], "allowed_by": i["allowed_by"]})
	}
	for _, c := range docs(requirements["completion_conditions"]) {
		item := clone(c)
		item["verification"] = nil
		for _, t := range docs(plan["tests"]) {
			if t["criterion"] == c["id"] {
				item["verification"] = t
				break
			}
		}
		checked = append(checked, item)
	}
	for _, j := range judgments {
		jj = append(jj, doc{"judge": j["judge"], "verdict": j["verdict"], "observations": j["observations"]})
		for _, f := range docs(j["findings"]) {
			if !truth(f["blocking"]) {
				item := doc{"judge": j["judge"]}
				for k, v := range f {
					item[k] = v
				}
				remaining = append(remaining, item)
			}
		}
	}
	remaining = append(remaining, carried...)
	for _, t := range docs(plan["tests"]) {
		if t["method"] != "test" {
			notChecked = append(notChecked, t)
		}
	}
	for _, d := range docs(plan["decisions"]) {
		if d["decided_by"] == "owner" || d["decided_by"] == "principle" || d["decided_by"] == "provisional" {
			decisions = append(decisions, d)
		}
	}
	return doc{"changed": changed, "checked": checked, "judgments": jj, "remaining": remaining, "not_checked": notChecked, "decisions": decisions, "answers": answers}
}
func facts(args []string, env cli.Env) error {
	ctx, err := context(env)
	if err != nil {
		return err
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch)
	if err != nil {
		return err
	}
	out := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr")
	if err = os.MkdirAll(env.Path(out), 0755); err != nil {
		return err
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	base, err := g.MergeBase(head)
	if err != nil {
		return err
	}
	h, err := record.History(ctx.Dir)
	if err != nil {
		return err
	}
	valid, err := record.ValidResults(h, "")
	if err != nil {
		return err
	}
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return err
	}
	plan, err := record.Latest(ctx.Dir, "plan")
	if err != nil {
		return err
	}
	accepted, err := record.PlanAccepted(ctx.Dir, plan, valid)
	if err != nil {
		return err
	}
	input := inputsOf(requirements, plan, valid, accepted)
	reasons := stringsOf(input["reasons"])
	clean, err := g.Clean()
	if err != nil {
		return err
	}
	if !clean {
		reasons = append(reasons, "the working tree has changes that are not committed")
	}
	if base == "" {
		reasons = append(reasons, "HEAD has no merge base with the base branch")
	} else if base == head {
		reasons = append(reasons, "the branch has no commit beyond the base branch")
	}
	judgments, err := record.List(ctx.Dir, "judgment")
	if err != nil {
		return err
	}
	id, err := g.ChangeID(head)
	if err != nil {
		return err
	}
	chain := verifyChain(judgments, head, object(input["versions"]), id)
	reasons = append(reasons, stringsOf(chain["reasons"])...)
	items := []doc{}
	if base != "" {
		changes, err := g.Changes(base, head)
		if err != nil {
			return err
		}
		items, err = itemsOf(g, base, head, c, changes)
		if err != nil {
			return err
		}
		if err = os.WriteFile(env.Path(filepath.Join(out, "diff.patch")), []byte(changes.Patch), 0644); err != nil {
			return err
		}
	}
	if err = save(env.Path(filepath.Join(out, "changes.json")), items); err != nil {
		return err
	}
	allowed, err := record.List(ctx.Dir, "allowed")
	if err != nil {
		return err
	}
	owner := asks(items, allowed, docs(object(input["plan"])["decisions"])...)
	questions := []doc{}
	for _, i := range owner {
		if i["allowed_by"] == nil {
			questions = append(questions, question(i))
		}
	}
	ready := len(reasons) == 0 && len(questions) == 0
	outcome := "ready"
	if len(reasons) > 0 {
		outcome = "stop"
		err = save(env.Path(filepath.Join(out, "outcome.json")), stopDoc("The prerequisites for opening a pull request did not hold", reasons))
	} else if len(questions) > 0 {
		outcome = "question"
		err = save(env.Path(filepath.Join(out, "outcome.json")), doc{"outcome": "question", "questions": questions})
	}
	if err != nil {
		return err
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return err
	}
	var request doc
	if len(requests) > 0 {
		request = requests[0]
	}
	chainJudgments := []doc{}
	for _, j := range judgments {
		if j["kind"] != "code" {
			continue
		}
		for _, s := range docs(chain["steps"]) {
			if s["run_id"] == j["run_id"] && s["commit"] == object(j["target"])["commit"] {
				chainJudgments = append(chainJudgments, j)
				break
			}
		}
	}
	paths := []string{}
	var judgeMaterials, writerMaterials concerns.Material
	if ready {
		if _, err = criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: text(request["criteria_commit"]), Dir: env.Path(filepath.Join(out, "criteria"))}); err != nil {
			return err
		}
		mapping, err := concerns.Load(env.Path(filepath.Join(out, "criteria", c.Standards)))
		if err != nil {
			return err
		}
		judgeMaterials = mapping.Materials(filepath.Join(out, "criteria", c.Standards), "judge:pr-body", "code")
		writerMaterials = mapping.Materials(filepath.Join(out, "criteria", c.Standards), "writer:pr-body", "code")
		paths = judgeMaterials.Criteria
		if err = save(env.Path(filepath.Join(out, "requirements.json")), doc{"requirements": requirements, "plan": input["plan"]}); err != nil {
			return err
		}
		if err = save(env.Path(filepath.Join(out, "judgments.json")), chainJudgments); err != nil {
			return err
		}
		allowedItems := []doc{}
		for _, i := range items {
			item := clone(i)
			item["allowed_by"] = nil
			for _, a := range owner {
				if a["category"] == i["category"] && a["file"] == i["file"] && a["item"] == i["item"] {
					item["allowed_by"] = a["allowed_by"]
					break
				}
			}
			allowedItems = append(allowedItems, item)
		}
		carried := []doc{}
		for _, r := range valid {
			carried = append(carried, docs(r["carried"])...)
		}
		answers, err := record.ListKeyed(ctx.Dir, "answer")
		if err != nil {
			return err
		}
		if err = save(env.Path(filepath.Join(out, "lists.json")), checkpoints(requirements, object(input["plan"]), answers, allowedItems, chainJudgments, carried)); err != nil {
			return err
		}
	}
	versions, err := record.LatestVersions(ctx.Dir)
	if err != nil {
		return err
	}
	indexed := []doc{}
	for i, q := range questions {
		d := clone(q)
		d["index"] = i
		indexed = append(indexed, d)
	}
	var baseValue any
	if base != "" {
		baseValue = base
	}
	if err = save(env.Path(filepath.Join(out, "facts.json")), doc{"language": c.Language, "standards": c.Standards, "head": head, "base": baseValue, "branch": ctx.Branch, "versions": input["versions"], "recorded_versions": versions, "criteria_commit": request["criteria_commit"], "criteria": paths, "judge_materials": judgeMaterials, "writer_materials": writerMaterials, "criteria_folder": filepath.Join(out, "criteria", c.Standards), "chain": chain, "outcome": outcome, "questions": indexed}); err != nil {
		return err
	}
	return env.JSON(doc{"ready": ready, "outcome": outcome, "reasons": reasons, "question_count": len(questions), "head": head})
}
