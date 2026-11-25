package optimize

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
)

func round(ctx record.ContextData, artifacts string, judgment any) (doc, error) {
	draft, err := readDraft(artifacts)
	if err != nil {
		return nil, err
	}
	checked, err := readJSON(file(artifacts, "check.json"))
	if err != nil {
		return nil, err
	}
	problems := stringsOf(checked["errors"])
	if checked["valid"] != true && len(problems) == 0 {
		problems = append(problems, "machine check did not pass")
	}
	committed, p, err := proposal.CommittedErrors(ctx, draft)
	if err != nil {
		return nil, err
	}
	problems = append(problems, committed...)
	je, err := record.ValidateSchema(file(artifacts, "judgment.schema.json"), judgment)
	if err != nil {
		return nil, err
	}
	j := obj(judgment)
	if len(je) == 0 {
		for _, aspect := range aspects {
			count := 0
			for _, c := range items(j["criteria"]) {
				if c["aspect"] == aspect {
					count++
				}
			}
			if count != 1 {
				je = append(je, aspect+": exactly one criterion is required")
			}
		}
	}
	blocking, missing := 0, 0
	if len(je) > 0 {
		blocking, missing = 1, 1
	} else {
		for _, f := range items(j["findings"]) {
			if f["blocking"] == true || f["needs_decision"] == true {
				blocking++
			}
		}
		for _, c := range items(j["criteria"]) {
			if c["verdict"] == "fail" {
				blocking++
			}
			if c["verdict"] == "not_assessed" {
				missing++
			}
		}
	}
	problems = append(problems, je...)
	previous, err := readJSON(file(artifacts, "rounds/latest.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	n := num(previous["round"]) + 1
	if n > 3 {
		return nil, fmt.Errorf("optimize is limited to three rounds")
	}
	approval, err := os.ReadFile(file(artifacts, "approval.txt"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	entry := doc{"round": n, "draft": draft, "judgment": judgment, "errors": problems, "cleared": len(problems) == 0 && blocking == 0 && missing == 0, "proposal_version": p["version"], "commit": obj(p["content"])["commit"], "approval_text": string(approval)}
	if len(committed) > 0 {
		entry["proposal_version"], entry["commit"] = nil, nil
	}
	if _, err = record.ProposalRoundTarget(entry, file(artifacts, fmt.Sprintf("rounds/%d.json", n))); err != nil {
		return nil, err
	}
	for _, name := range []string{fmt.Sprintf("rounds/%d.json", n), "rounds/latest.json"} {
		if err = writeJSON(file(artifacts, name), entry); err != nil {
			return nil, err
		}
	}
	input, err := readJSON(file(artifacts, "input.json"))
	if err != nil {
		return nil, err
	}
	input["next_proposal_version"] = num(p["version"]) + 1
	if err = writeJSON(file(artifacts, "input.json"), input); err != nil {
		return nil, err
	}
	return doc{"round": n, "valid": len(problems) == 0, "blocking_count": blocking, "not_assessed_count": missing}, nil
}

func resultOptions(ctx record.ContextData) (record.ResultOptions, error) {
	base, err := gitx.Run(ctx.Cwd, "merge-base", "HEAD", ctx.BaseBranch)
	if err != nil {
		return record.ResultOptions{}, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	return record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head}, err
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
	return doc{"outcome": result["outcome"], "proposal_version": version, "issue_numbers": numbers, "pr_number": num(result["pr_number"]), "pr_url": str(result["pr_url"]), "publication": str(result["publication"]), "questions": qs, "reasons": stringsOf(obj(result["stop"])["unresolved"])}
}
func saveResult(ctx record.ContextData, result doc, version int) (doc, error) {
	o, err := resultOptions(ctx)
	if err != nil {
		return nil, err
	}
	if version > 0 {
		result["proposal_version"] = version
	}
	r, err := record.WriteResult(ctx.Dir, "optimize", result, o)
	if err != nil {
		return nil, err
	}
	return output(r, version), nil
}
func finish(ctx record.ContextData, artifacts string) (doc, error) {
	entry, err := readJSON(file(artifacts, "rounds/latest.json"))
	if err != nil {
		return nil, err
	}
	input, err := readJSON(file(artifacts, "input.json"))
	if err != nil {
		return nil, err
	}
	problems, p, err := proposal.CommittedErrors(ctx, entry["draft"])
	if err != nil {
		return nil, err
	}
	draft, err := readDraft(artifacts)
	if err != nil {
		return nil, err
	}
	approval, err := os.ReadFile(file(artifacts, "approval.txt"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if !reflect.DeepEqual(draft, entry["draft"]) || string(approval) != str(entry["approval_text"]) || p["version"] != entry["proposal_version"] || obj(p["content"])["commit"] != entry["commit"] {
		problems = append(problems, "draft or approval text changed after judgment")
	}
	records := []doc{}
	for n := 1; n <= num(entry["round"]); n++ {
		path := file(artifacts, fmt.Sprintf("rounds/%d.json", n))
		r, err := readJSON(path)
		if err != nil {
			return nil, err
		}
		target, err := record.ProposalRoundTarget(r, path)
		if err != nil {
			return nil, err
		}
		observed := stringsOf(r["errors"])
		findings := []doc{}
		verdict := "pass"
		je, err := record.ValidateSchema(file(artifacts, "judgment.schema.json"), r["judgment"])
		if err != nil {
			return nil, err
		}
		if len(je) > 0 || len(observed) > 0 || target["proposal_version"] == nil {
			verdict = "not_assessed"
			observed = append(observed, je...)
		} else {
			j := obj(r["judgment"])
			for _, c := range items(j["criteria"]) {
				observed = append(observed, str(c["aspect"])+": "+str(c["checked"]))
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
				findings = append(findings, doc{"id": fmt.Sprintf("optimize-%d", i+1), "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": fmt.Sprintf("%s %s: %s; %s", f["aspect"], f["item"], f["failure_scenario"], f["fix"]), "evidence": f["evidence"], "mechanical": f["mechanical"], "resolution_key": str(f["aspect"]) + "\n" + str(f["item"]) + "\n" + str(f["failure_scenario"])})
			}
		}
		records = append(records, doc{"judge": "optimize", "phase": "optimize", "round": n, "kind": "proposal", "target": target, "materials": []string{file(artifacts, "input.json"), path, str(input["criteria"])}, "observations": observed, "verdict": verdict, "findings": findings})
	}
	if err = record.WriteLoopJudgments(ctx, records, ""); err != nil {
		return nil, err
	}
	if entry["cleared"] != true || len(problems) > 0 {
		problems = append(problems, stringsOf(entry["errors"])...)
		if len(problems) == 0 {
			problems = append(problems, "The optimize draft did not clear review")
		}
		if p != nil && p["run_id"] == ctx.RunID {
			content := obj(p["content"])
			content["ready"] = false
			if err = proposal.Save(ctx, p, content); err != nil {
				return nil, err
			}
		}
		return saveResult(ctx, doc{"outcome": "stop", "stop": doc{"state": "The optimize draft did not clear review", "unresolved": problems, "premise": ""}}, num(p["version"]))
	}
	content := obj(p["content"])
	d := obj(entry["draft"])
	text := str(d["summary"])
	for _, point := range stringsOf(d["owner_points"]) {
		text += "\n\n" + point
	}
	text += "\n\n" + strings.TrimSpace(string(approval)) + proposal.ApprovalPRText(d, str(input["language"]))
	text += fmt.Sprintf("\n\nProposal version: %d\nCommit: %s", num(p["version"]), str(content["commit"]))
	content["ready"], content["approval_text"], content["judgment"] = true, text, entry["judgment"]
	if err = proposal.Save(ctx, p, content); err != nil {
		return nil, err
	}
	return saveResult(ctx, doc{"outcome": "question", "questions": []doc{{"text": text, "target": doc{"kind": "proposal", "ref": strconv.Itoa(num(p["version"]))}, "next_workflows": []string{"optimize"}, "invalidates": []string{}}}}, num(p["version"]))
}
