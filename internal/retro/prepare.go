package retro

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func prepare(ctx record.ContextData, artifacts string) (doc, error) {
	display := artifacts
	artifacts = resolve(ctx.Cwd, artifacts)
	c, e := config.Load(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	entries, backlog, e := readBacklog(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	requests, e := record.List(ctx.Dir, "request")
	if e != nil {
		return nil, e
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("retro must pass start before prepare")
	}
	_, e = criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: str(requests[0]["criteria_commit"]), Dir: filepath.Join(artifacts, "criteria")})
	if e != nil {
		return nil, e
	}
	fixed := filepath.Join(artifacts, "criteria", c.Standards)
	mapping, e := concerns.Load(fixed)
	if e != nil {
		return nil, e
	}
	index, e := os.ReadFile(filepath.Join(fixed, "README.md"))
	if e != nil {
		return nil, e
	}
	previous, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	answers, e := record.ListKeyed(ctx.Dir, "answer")
	if e != nil {
		return nil, e
	}
	data, e := decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	fixedDisplay := filepath.Join(display, "criteria", c.Standards)
	input := doc{"units": data.Units, "entries": entries, "backlog": backlog, "config": c, "concerns": mapping, "standards_index": string(index), "criteria": fixedDisplay, "judge_materials": mapping.Materials(fixedDisplay, "judge:retro", "plan"), "writer_materials": mapping.Materials(fixedDisplay, "writer:retro", "plan"), "language": c.Language, "folders": doc{"units": c.Units, "standards": c.Standards}, "answers": answers, "previous": previous, "next_proposal_version": num(previous["version"]) + 1}
	if e = writeJSON(file(artifacts, "input.json"), input); e != nil {
		return nil, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
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
