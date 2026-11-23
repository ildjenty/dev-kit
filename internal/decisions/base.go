package decisions

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/gitx"
)

type Citation struct {
	ID    string   `json:"id"`
	Stage string   `json:"stage"`
	Items []string `json:"items"`
}

type DecisionText struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type SupersededDecision struct {
	DecisionText
	SupersededBy []string       `json:"superseded_by"`
	Successors   []DecisionText `json:"successors"`
	Citations    []Citation     `json:"citations"`
}

type BaseCheck struct {
	Base       string               `json:"base"`
	Superseded []SupersededDecision `json:"superseded"`
	Target     string               `json:"target,omitempty"`
	Note       string               `json:"note,omitempty"`
}

func (check BaseCheck) Reasons() []string {
	reasons := []string{}
	for _, d := range check.Superseded {
		stage := "plan"
		for _, cite := range d.Citations {
			if cite.Stage == "requirements" {
				stage = "requirements"
			}
		}
		by := strings.Join(d.SupersededBy, ", ")
		if by == "" {
			by = "unknown (no successor recorded)"
		}
		reasons = append(reasons, fmt.Sprintf("decision %s was superseded on %s by %s and this branch's %s rests on it: run sync on this branch, then start %s", d.ID, check.Base, by, stage, check.Target))
	}
	return reasons
}

func (check BaseCheck) Refuses(workflow string) bool {
	stages := []string{"requirements", "plan", "implement", "judge", "fix", "pr"}
	target, current := slices.Index(stages, check.Target), slices.Index(stages, workflow)
	return target >= 0 && current > target
}

func decisionText(item Item, id string) DecisionText {
	return DecisionText{ID: id, Title: text(item["title"]), Body: text(item["body"])}
}

func CheckBase(cwd, base string, ids []string) (BaseCheck, error) {
	if base == "" {
		base = "main"
	}
	check := BaseCheck{Base: base, Superseded: []SupersededDecision{}}
	g, err := gitx.New(cwd, base)
	if err != nil {
		check.Note = fmt.Sprintf("decision check skipped: neither origin/%s nor %s is available locally", base, base)
		return check, nil
	}
	check.Base = g.Refs[0]
	settings := struct {
		Decisions string `json:"decisions"`
	}{Decisions: ".devkit/decisions"}
	if g.OK("cat-file", "-e", check.Base+":.devkit/config.json") {
		data, err := g.Bytes("show", check.Base+":.devkit/config.json")
		if err != nil {
			return check, err
		}
		if err = json.Unmarshal(data, &settings); err != nil {
			return check, err
		}
	}
	read := func(path string) (Item, error) {
		data, err := g.Bytes("show", check.Base+":"+path)
		if err != nil {
			return nil, err
		}
		item, err := Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s:%s: %w", check.Base, path, err)
		}
		return item, nil
	}
	for _, id := range ids {
		path := filepath.ToSlash(filepath.Join(settings.Decisions, id+".md"))
		if !g.OK("cat-file", "-e", check.Base+":"+path) {
			continue
		}
		d, err := read(path)
		if err != nil {
			return check, err
		}
		if d["status"] == "superseded" {
			check.Superseded = append(check.Superseded, SupersededDecision{DecisionText: decisionText(d, id), SupersededBy: []string{}, Successors: []DecisionText{}, Citations: []Citation{}})
		}
	}
	if len(check.Superseded) == 0 {
		return check, nil
	}
	paths, err := g.Run("ls-tree", "-r", "--name-only", "-z", check.Base, "--", settings.Decisions)
	if err != nil {
		return check, err
	}
	for _, path := range strings.Split(paths, "\x00") {
		if !regexp.MustCompile(`^D-[0-9a-f]{6,64}\.md$`).MatchString(filepath.Base(path)) {
			continue
		}
		d, err := read(path)
		if err != nil {
			return check, err
		}
		for i := range check.Superseded {
			if has(d["supersedes"], check.Superseded[i].ID) {
				id := strings.TrimSuffix(filepath.Base(path), ".md")
				check.Superseded[i].SupersededBy = append(check.Superseded[i].SupersededBy, id)
				check.Superseded[i].Successors = append(check.Superseded[i].Successors, decisionText(d, id))
			}
		}
	}
	return check, nil
}
