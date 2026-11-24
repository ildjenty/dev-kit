package retro

import (
	"fmt"
	"os"

	"github.com/ildjenty/dev-kit/internal/record"
)

func round(ctx record.ContextData, artifacts string, judgment any) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	d, e := readDraft(artifacts)
	if e != nil {
		return nil, e
	}
	errors, e := draftErrors(ctx, artifacts, d)
	if e != nil {
		return nil, e
	}
	committed, proposal, e := committedErrors(ctx, d)
	if e != nil {
		return nil, e
	}
	errors = append(errors, committed...)
	je, e := schema("judgment", judgment)
	if e != nil {
		return nil, e
	}
	j := obj(judgment)
	if len(je) == 0 {
		for _, aspect := range aspects {
			n := 0
			for _, c := range items(j["criteria"]) {
				if c["aspect"] == aspect {
					n++
				}
			}
			if n != 1 {
				je = append(je, aspect+": exactly one criterion is required")
			}
		}
	}
	previous, e := readJSON(file(artifacts, "rounds/latest.json"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	n := num(previous["round"]) + 1
	if n > 3 {
		return nil, fmt.Errorf("retro is limited to three rounds")
	}
	errors = append(errors, je...)
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
	approval, e := os.ReadFile(file(artifacts, "approval.txt"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	entry := doc{"approval_text": string(approval), "commit": obj(proposal["content"])["commit"], "proposal_version": proposal["version"], "round": n, "draft": d, "judgment": judgment, "errors": errors, "cleared": len(errors) == 0 && blocking == 0 && missing == 0}
	if e = writeJSON(file(artifacts, fmt.Sprintf("rounds/%d.json", n)), entry); e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "rounds/latest.json"), entry); e != nil {
		return nil, e
	}
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	input["next_proposal_version"] = num(proposal["version"]) + 1
	if e = writeJSON(file(artifacts, "input.json"), input); e != nil {
		return nil, e
	}
	return doc{"round": n, "valid": len(errors) == 0, "blocking_count": blocking, "not_assessed_count": missing}, nil
}
