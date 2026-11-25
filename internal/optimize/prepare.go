package optimize

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
)

//go:embed schemas/*.json
var schemas embed.FS

type Draft struct {
	Summary     string              `json:"summary"`
	OwnerPoints []string            `json:"owner_points"`
	PRTitle     string              `json:"pr_title"`
	PRSummary   string              `json:"pr_summary"`
	Changes     []Change            `json:"changes"`
	Triage      []issues.TriageItem `json:"triage"`
}
type Change struct {
	Path     string `json:"path"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}

func prepare(ctx record.ContextData, env cli.Env, artifacts string) (doc, error) {
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("optimize must pass start before prepare")
	}
	instructions := str(requests[len(requests)-1]["owner_words"])
	if text := env.Getenv("INPUTS_INSTRUCTIONS"); text != "" {
		instructions = text
		if _, err = record.Write(ctx, "request", doc{"owner_words": text, "supplement": ""}, ""); err != nil {
			return nil, err
		}
	}
	source, err := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: str(requests[0]["criteria_commit"]), Dir: filepath.Join(resolve(ctx.Cwd, artifacts), "criteria")})
	if err != nil {
		return nil, err
	}
	fixed := filepath.Join(source.Dir, source.Paths[0])
	if _, err = concerns.Load(fixed); err != nil {
		return nil, err
	}
	existing, err := (issues.Client{Env: env}).Untriaged(c.Issues.Labels)
	if err != nil {
		return nil, err
	}
	answers, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return nil, err
	}
	previous, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	base := str(obj(previous["content"])["base_commit"])
	if base == "" {
		base = head
	}
	materials := []string{}
	for _, name := range []string{"README.md", "harness.md", "review/harness.md", "authority.md", "review/purpose.md", "review/README.md"} {
		materials = append(materials, filepath.Join(fixed, name))
	}
	input := doc{"instructions": instructions, "answers": answers, "previous": previous, "config": c, "language": c.Language, "criteria": fixed, "materials": materials, "folders": doc{"standards": c.Standards}, "issues": existing, "proposal_base": base, "base_branch": ctx.BaseBranch, "next_proposal_version": num(previous["version"]) + 1}
	if err = writeJSON(file(resolve(ctx.Cwd, artifacts), "input.json"), input); err != nil {
		return nil, err
	}
	if err = writeJSON(file(resolve(ctx.Cwd, artifacts), "baseline.json"), doc{"commit": head}); err != nil {
		return nil, err
	}
	for _, name := range []string{"draft", "judgment"} {
		data, e := schemas.ReadFile("schemas/" + name + ".schema.json")
		if e != nil {
			return nil, e
		}
		if err = put(file(resolve(ctx.Cwd, artifacts), name+".schema.json"), string(data)); err != nil {
			return nil, err
		}
	}
	return doc{"input": file(artifacts, "input.json"), "criteria": fixed}, nil
}

func readTypedDraft(artifacts string) (Draft, doc, error) {
	value, err := readJSON(file(artifacts, "draft.json"))
	if err != nil {
		return Draft{}, nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return Draft{}, nil, err
	}
	var draft Draft
	err = json.Unmarshal(data, &draft)
	return draft, value, err
}
