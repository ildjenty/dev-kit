package retro

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func prepare(ctx record.ContextData, artifacts string) (doc, error) {
	display := artifacts
	artifacts = resolve(ctx.Cwd, artifacts)
	previous, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	if oldProposal(previous) {
		return nil, fmt.Errorf("old-format retro proposal cannot be used; start retro again on a new retro branch")
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	base, root := head, ctx.Cwd
	if commit := str(obj(previous["content"])["base_commit"]); commit != "" {
		base = commit
		root, e = os.MkdirTemp("", "devkit-retro-base-*")
		if e != nil {
			return nil, e
		}
		defer os.RemoveAll(root)
		g, e := gitx.New(ctx.Cwd, ctx.BaseBranch)
		if e != nil {
			return nil, e
		}
		if e = g.ExportTree(base, root); e != nil {
			return nil, e
		}
	}
	configData, e := os.ReadFile(filepath.Join(root, ".devkit/config.json"))
	if os.IsNotExist(e) {
		configData = []byte("{}")
	} else if e != nil {
		return nil, e
	}
	c, e := config.Parse(configData)
	if e != nil {
		return nil, e
	}
	entries, backlog, e := readBacklog(root)
	if e != nil {
		return nil, e
	}
	if root != ctx.Cwd {
		merge := func(root string) error {
			current, files, e := readBacklog(root)
			if e != nil {
				return e
			}
			for _, entry := range current {
				if backlog[str(entry["path"])] == nil {
					entries = append(entries, entry)
				}
			}
			for path, body := range files {
				if backlog[path] == nil {
					backlog[path] = body
				}
			}
			return nil
		}
		proposals, e := record.List(ctx.Dir, "proposal")
		if e != nil {
			return nil, e
		}
		g, e := gitx.New(ctx.Cwd, ctx.BaseBranch)
		if e != nil {
			return nil, e
		}
		seen := map[string]bool{base: true, head: true}
		for _, proposal := range proposals {
			commit := str(obj(proposal["content"])["commit"])
			if commit == "" {
				continue
			}
			parent, e := gitx.Run(ctx.Cwd, "rev-parse", commit+"^")
			if e != nil {
				return nil, e
			}
			if seen[parent] {
				continue
			}
			seen[parent] = true
			if e = g.ExportTree(parent, root); e != nil {
				return nil, e
			}
			if e = merge(root); e != nil {
				return nil, e
			}
		}
		if e = merge(ctx.Cwd); e != nil {
			return nil, e
		}
	}
	requests, e := record.List(ctx.Dir, "request")
	if e != nil {
		return nil, e
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("retro must pass start before prepare")
	}
	source, e := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: str(requests[0]["criteria_commit"]), Dir: filepath.Join(artifacts, "criteria")})
	if e != nil {
		return nil, e
	}
	fixed := filepath.Join(source.Dir, source.Paths[0])
	mapping, e := concerns.Load(fixed)
	if e != nil {
		return nil, e
	}
	index, e := os.ReadFile(filepath.Join(fixed, "README.md"))
	if e != nil {
		return nil, e
	}
	answers, e := record.ListKeyed(ctx.Dir, "answer")
	if e != nil {
		return nil, e
	}
	fixedDisplay := filepath.Join(display, "criteria", source.Paths[0])
	input := doc{"entries": entries, "backlog": backlog, "config": c, "concerns": mapping, "standards_index": string(index), "criteria": fixedDisplay, "judge_materials": mapping.Materials(fixedDisplay, "judge:retro", "plan"), "writer_materials": mapping.Materials(fixedDisplay, "writer:retro", "plan"), "language": c.Language, "folders": doc{"standards": c.Standards, "eval": c.Eval}, "answers": answers, "previous": previous, "next_proposal_version": num(previous["version"]) + 1}
	input["proposal_base"] = base
	input["branch"] = ctx.Branch
	if e = writeJSON(file(artifacts, "input.json"), input); e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "baseline.json"), doc{"commit": head}); e != nil {
		return nil, e
	}
	for _, name := range []string{"draft", "judgment"} {
		b, e := schemas.ReadFile("schemas/" + name + ".schema.json")
		if e != nil {
			return nil, e
		}
		if e = put(file(artifacts, name+".schema.json"), string(b)); e != nil {
			return nil, e
		}
	}
	return doc{"input": file(display, "input.json"), "criteria": fixedDisplay, "empty": len(entries) == 0}, nil
}
