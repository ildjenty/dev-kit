package record

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/unitdone"
)

func BranchDecisionCitations(dir, cwd, ref string) ([]decisions.Citation, error) {
	citations := []decisions.Citation{}
	add := func(value any, stage, item string) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		for _, id := range decisions.Citations(string(data)) {
			index := slices.IndexFunc(citations, func(c decisions.Citation) bool { return c.ID == id && c.Stage == stage })
			if index < 0 {
				citations = append(citations, decisions.Citation{ID: id, Stage: stage, Items: []string{item}})
			} else if !slices.Contains(citations[index].Items, item) {
				citations[index].Items = append(citations[index].Items, item)
			}
		}
		return nil
	}
	unit, err := unitdone.RequestUnit(dir)
	if err != nil {
		return nil, err
	}
	if unit != "" {
		read := func(path string) ([]byte, error) {
			if ref == "" {
				return os.ReadFile(filepath.Join(cwd, path))
			}
			g := gitx.Git{Cwd: cwd}
			if !g.OK("cat-file", "-e", ref+":"+path) {
				return nil, os.ErrNotExist
			}
			return g.Bytes("show", ref+":"+path)
		}
		settings := struct {
			Units string `json:"units"`
		}{Units: config.Defaults("").Units}
		if data, err := read(".devkit/config.json"); err == nil {
			if err = json.Unmarshal(data, &settings); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		data, err := read(filepath.ToSlash(filepath.Join(settings.Units, unit+".md")))
		if err == nil {
			item, err := decisions.Parse(string(data))
			if err != nil {
				return nil, err
			}
			if err = add(item["decisions"], "requirements", unit); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	for _, kind := range []string{"requirements", "plan"} {
		doc, err := Latest(dir, kind)
		if err != nil {
			return nil, err
		}
		fields := []string{"goal", "completion_conditions", "constraints", "open_points"}
		if kind == "plan" {
			fields = []string{"decisions"}
		}
		for _, field := range fields {
			if field == "goal" {
				if err = add(doc[field], kind, field); err != nil {
					return nil, err
				}
				continue
			}
			for i, value := range array(doc[field]) {
				item := str(object(value)["id"])
				if item == "" {
					item = fmt.Sprintf("%s[%d]", field, i+1)
				}
				if err = add(value, kind, item); err != nil {
					return nil, err
				}
			}
		}
	}
	slices.SortFunc(citations, func(a, b decisions.Citation) int {
		if a.ID != b.ID {
			return strings.Compare(a.ID, b.ID)
		}
		return strings.Compare(a.Stage, b.Stage)
	})
	return citations, nil
}

func CheckBranchDecisions(dir, cwd, ref, base string) (decisions.BaseCheck, error) {
	citations, err := BranchDecisionCitations(dir, cwd, ref)
	if err != nil {
		return decisions.BaseCheck{}, err
	}
	ids := []string{}
	for _, cite := range citations {
		if !slices.Contains(ids, cite.ID) {
			ids = append(ids, cite.ID)
		}
	}
	check, err := decisions.CheckBase(cwd, base, ids)
	if err != nil {
		return check, err
	}
	for i := range check.Superseded {
		for _, cite := range citations {
			if cite.ID != check.Superseded[i].ID {
				continue
			}
			check.Superseded[i].Citations = append(check.Superseded[i].Citations, cite)
			if check.Target == "" || cite.Stage == "requirements" {
				check.Target = cite.Stage
			}
		}
	}
	return check, nil
}
