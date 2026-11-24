package retro

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
)

func resultOptions(ctx record.ContextData) (record.ResultOptions, error) {
	base, e := gitx.Run(ctx.Cwd, "merge-base", "HEAD", ctx.BaseBranch)
	if e != nil {
		return record.ResultOptions{}, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	return record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head}, e
}
func output(result doc, version int) doc {
	qs := []doc{}
	for _, q := range items(result["questions"]) {
		qs = append(qs, doc{"id": q["id"], "text": q["text"]})
	}
	numbers := result["issue_numbers"]
	if numbers == nil {
		numbers = []int{}
	}
	return doc{"outcome": result["outcome"], "proposal_version": version, "issue_numbers": numbers, "questions": qs, "reasons": stringsOf(obj(result["stop"])["unresolved"])}
}
func preflight(ctx record.ContextData) error {
	o, e := resultOptions(ctx)
	if e != nil {
		return e
	}
	_, e = record.BuildResult(ctx.Dir, "retro", doc{"outcome": "pass"}, o)
	return e
}
func saveResult(ctx record.ContextData, result doc, version int) (doc, error) {
	o, e := resultOptions(ctx)
	if e != nil {
		return nil, e
	}
	if version > 0 {
		result["proposal_version"] = version
	}
	r, e := record.WriteResult(ctx.Dir, "retro", result, o)
	if e != nil {
		return nil, e
	}
	return output(r, version), nil
}
func empty(ctx record.ContextData, artifacts string) (doc, error) {
	if e := preflight(ctx); e != nil {
		return nil, e
	}
	input, e := readJSON(file(resolve(ctx.Cwd, artifacts), "input.json"))
	if e != nil {
		return nil, e
	}
	if len(items(input["entries"])) != 0 {
		return nil, fmt.Errorf("backlog is not empty")
	}
	p, e := record.Write(ctx, "proposal", doc{"content": doc{"draft": doc{"summary": "No backlog entries", "units": []doc{}, "checks": []doc{}, "standards": []doc{}, "dismissed": []doc{}}, "ready": false}}, "")
	if e != nil {
		return nil, e
	}
	return saveResult(ctx, doc{"outcome": "pass"}, num(p.Entry["version"]))
}
func finish(ctx record.ContextData, artifacts string) (doc, error) {
	if e := preflight(ctx); e != nil {
		return nil, e
	}
	artifacts = resolve(ctx.Cwd, artifacts)
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	entry, e := readJSON(file(artifacts, "rounds/latest.json"))
	if e != nil {
		return nil, e
	}
	errors, e := reviewedErrors(ctx, artifacts, entry)
	if e != nil {
		return nil, e
	}
	p, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	content := obj(p["content"])
	if content != nil {
		content["ready"] = false
	}
	approvalText := str(entry["approval_text"])
	if entry["cleared"] == true && len(errors) == 0 && len(items(obj(entry["draft"])["cases"])) > 0 {
		verification, e := readJSON(file(artifacts, "verification.json"))
		if e != nil || verification["review"] != reviewHash(entry) {
			errors = append(errors, "escape evaluation must verify the reviewed draft before approval")
		} else {
			report, e := readJSON(file(artifacts, "verification-text.json"))
			if e != nil || strings.TrimSpace(str(report["text"])) == "" {
				errors = append(errors, "escape evaluation report is required in the configured language")
			} else {
				content["verification"] = verification
				approvalText += "\n\n" + str(report["text"])
			}
		}
	}
	result := doc{}
	if entry["cleared"] != true || len(errors) > 0 {
		if len(errors) == 0 {
			errors = append(errors, "The retro draft did not clear review")
		}
		result = doc{"outcome": "stop", "stop": doc{"state": "The retro draft did not clear review", "unresolved": errors, "premise": ""}}
	} else {
		approvalText = fmt.Sprintf("%s\n\nProposal version: %d\nCommit: %s", strings.TrimSpace(approvalText), num(p["version"]), str(obj(p["content"])["commit"]))
		content["approval_text"], content["judgment"], content["ready"] = approvalText, entry["judgment"], true
		result = doc{"outcome": "question", "questions": []doc{{"text": approvalText, "target": doc{"kind": "proposal", "ref": strconv.Itoa(num(p["version"]))}, "next_workflows": []string{"retro"}, "invalidates": []string{}}}}
	}
	if p == nil || p["run_id"] != ctx.RunID && str(obj(p["content"])["commit"]) == "" {
		written, e := record.Write(ctx, "proposal", doc{"content": doc{"draft": entry["draft"], "ready": false}}, "")
		if e != nil {
			return nil, e
		}
		p = written.Entry
	} else if p["run_id"] == ctx.RunID {
		if e = finishProposal(ctx, p, content); e != nil {
			return nil, e
		}
	}
	records := []doc{}
	for n := 1; n <= num(entry["round"]); n++ {
		roundEntry, e := readJSON(file(artifacts, fmt.Sprintf("rounds/%d.json", n)))
		if e != nil {
			return nil, e
		}
		version := roundEntry["proposal_version"]
		if num(version) == 0 {
			version = p["version"]
		}
		judgmentErrors, e := schema("judgment", roundEntry["judgment"])
		if e != nil {
			return nil, e
		}
		if len(judgmentErrors) == 0 {
			observations, findings, verdict := []string{}, []doc{}, "pass"
			j := obj(roundEntry["judgment"])
			for _, aspect := range aspects {
				count := 0
				for _, c := range items(j["criteria"]) {
					if c["aspect"] == aspect {
						count++
					}
				}
				if count != 1 {
					verdict = "not_assessed"
				}
			}
			for _, c := range items(j["criteria"]) {
				observations = append(observations, str(c["aspect"])+": "+str(c["checked"]))
				if c["verdict"] == "not_assessed" && verdict != "fail" {
					verdict = "not_assessed"
				}
				if c["verdict"] == "fail" {
					verdict = "fail"
				}
			}
			for i, f := range items(j["findings"]) {
				if f["blocking"] == true || f["needs_decision"] == true {
					verdict = "fail"
				}
				findings = append(findings, doc{"id": fmt.Sprintf("retro-%d", i+1), "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": fmt.Sprintf("%s %s: %s; %s", f["aspect"], f["item"], f["failure_scenario"], f["fix"]), "evidence": f["evidence"], "mechanical": f["mechanical"] == true, "deferred": f["deferred"] == true, "resolution_key": str(f["aspect"]) + "\n" + str(f["item"]) + "\n" + str(f["failure_scenario"])})
			}
			if errors := stringsOf(roundEntry["errors"]); len(errors) > 0 {
				verdict = "not_assessed"
				observations = append(observations, errors...)
			}
			records = append(records, doc{"judge": "retro", "phase": "retro", "round": n, "kind": "proposal", "target": doc{"proposal_version": version}, "materials": []string{file(artifacts, "input.json"), file(artifacts, fmt.Sprintf("rounds/%d.json", n)), str(input["criteria"])}, "observations": observations, "verdict": verdict, "findings": findings})
		} else {
			records = append(records, doc{"judge": "retro", "phase": "retro", "round": n, "kind": "proposal", "target": doc{"proposal_version": version}, "materials": []string{file(artifacts, fmt.Sprintf("rounds/%d.json", n))}, "observations": judgmentErrors, "verdict": "not_assessed", "findings": []doc{}})
		}
	}
	if e = record.WriteLoopJudgments(ctx, records, ""); e != nil {
		return nil, e
	}

	return saveResult(ctx, result, num(p["version"]))
}
func approve(ctx record.ContextData, version string, environments ...cli.Env) (doc, error) {
	if e := authorizedApproval(ctx, version); e != nil {
		return nil, e
	}
	if e := preflight(ctx); e != nil {
		return nil, e
	}
	status, e := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return nil, e
	}
	if status != "" {
		return nil, fmt.Errorf("approval requires a clean worktree")
	}
	p, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	content := obj(p["content"])
	c, e := config.Load(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	drafts, e := issues.ReadDrafts(content["draft"])
	if e != nil {
		return nil, e
	}
	env := cli.Env{Cwd: ctx.Cwd}
	if len(environments) > 0 {
		env = environments[0]
		env.Cwd = ctx.Cwd
	}
	path := filepath.Join(ctx.Dir, "issue-creation", version+".json")
	progress, e := issues.ReadCreation(path)
	if e != nil {
		return nil, e
	}
	if progress.RunID == "" {
		progress.RunID, progress.Commit = ctx.RunID, str(content["commit"])
		if e = issues.SaveCreation(path, progress); e != nil {
			return nil, e
		}
	}
	progress, e = (issues.Client{Env: env}).CreateDrafts(path, drafts, c.Issues.Labels, nil, 0, "")
	if e != nil {
		return nil, e
	}
	return saveResult(ctx, doc{"outcome": "pass", "issue_numbers": progress.Numbers()}, num(p["version"]))

}
