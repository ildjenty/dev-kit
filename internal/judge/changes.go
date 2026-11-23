package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
)

func covers(touch, p string) bool {
	return config.Covers(touch, p)
}
func decisionDocs(c config.Config, changes gitx.Changes) []doc {
	out := []doc{}
	for _, d := range config.DecisionChanges(c, changes) {
		b, _ := json.Marshal(d)
		var item doc
		_ = json.Unmarshal(b, &item)
		out = append(out, item)
	}
	return out
}
func summary(mapping concerns.Map, c config.Config, changes gitx.Changes, plan doc) doc {
	decisions := decisionDocs(c, changes)
	unlisted := []doc{}
	rows := docs(plan["decisions"])
	if plan != nil {
		for _, change := range decisions {
			found := false
			for _, r := range rows {
				for _, touch := range stringsOf(r["touches"]) {
					if covers(touch, text(change["path"])) {
						found = true
					}
				}
			}
			if !found {
				unlisted = append(unlisted, change)
			}
		}
	}
	launch := map[string]bool{"scope": true}
	for _, q := range config.QualitiesFor(c, changes) {
		launch[q] = true
		if q == "test" || q == "readability" {
			launch["readability"] = true
			launch["test"] = true
		}
	}
	for _, r := range rows {
		launch[text(r["quality"])] = true
	}
	result := doc{"decisions": decisions, "unlisted": unlisted, "proceed": len(unlisted) == 0, "changes_count": len(decisions), "unlisted_count": len(unlisted)}
	for _, q := range mapping.Judged("code") {
		result["launch_"+q] = launch[q]
	}
	return result
}
func structural(status string) bool {
	return status != "" && strings.ContainsRune("ADRC", rune(status[0]))
}
func structureFacts(g *gitx.Git, base, commit string, changes gitx.Changes, plans ...doc) (doc, error) {
	folders := map[string]bool{}
	files := []gitx.File{}
	for _, f := range changes.Files {
		for _, p := range []string{f.Path, f.OldPath} {
			if p != "" {
				folder := path.Dir(p)
				folders[folder] = true
			}
		}
		if structural(f.Status) {
			files = append(files, f)
		}
	}
	initial := []string{}
	for f := range folders {
		initial = append(initial, f)
	}
	for _, f := range initial {
		for parent := path.Dir(f); ; parent = path.Dir(parent) {
			folders[parent] = true
			if parent == "." {
				break
			}
		}
	}
	for _, f := range append(append([]string{}, changes.AddedDirectories...), changes.RemovedDirectories...) {
		folders[f] = true
		for parent := path.Dir(f); ; parent = path.Dir(parent) {
			folders[parent] = true
			if parent == "." {
				break
			}
		}
	}
	sorted := []string{}
	for f := range folders {
		sorted = append(sorted, f)
	}
	sort.Strings(sorted)
	siblings := []doc{}
	for _, folder := range sorted {
		sibling := doc{"folder": folder}
		for side, ref := range map[string]string{"base": base, "commit": commit} {
			args := []string{"ls-tree", "-z", ref}
			if folder != "." {
				args = append(args, folder+"/")
			}
			out, err := g.Run(args...)
			if err != nil {
				return nil, err
			}
			listing := []string{}
			for _, s := range strings.Split(out, "\x00") {
				if s != "" {
					listing = append(listing, s)
				}
			}
			sibling[side] = listing
		}
		siblings = append(siblings, sibling)
	}
	outside := []string{}
	top, second := map[string]bool{}, map[string]bool{}
	var rows []doc
	if len(plans) > 0 {
		rows = docs(plans[0]["decisions"])
	}
	for _, file := range changes.Files {
		for _, name := range []string{file.Path, file.OldPath} {
			if name == "" {
				continue
			}
			parts := strings.Split(name, "/")
			if len(parts) > 1 {
				top[parts[0]] = true
			}
			if len(parts) > 2 {
				second[strings.Join(parts[:2], "/")] = true
			}
			covered := false
			for _, row := range rows {
				for _, touch := range stringsOf(row["touches"]) {
					if covers(touch, name) {
						covered = true
					}
				}
			}
			if !covered && !slices.Contains(outside, name) {
				outside = append(outside, name)
			}
		}
	}
	sort.Strings(outside)
	return doc{"outside_plan": outside, "top_level_folders": len(top), "second_level_folders": len(second), "files": files, "addedDirectories": changes.AddedDirectories, "removedDirectories": changes.RemovedDirectories, "siblings": siblings}, nil
}
func writeChanges(root, artifacts string) (doc, error) {
	dir := filepath.Join(artifacts, "judge")
	target, err := read(filepath.Join(dir, "target.json"))
	if err != nil {
		return nil, err
	}
	c, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	g, err := gitx.New(root, text(target["base_branch"]))
	if err != nil {
		return nil, err
	}
	changes, err := g.Changes(text(target["base"]), text(target["commit"]))
	if err != nil {
		return nil, err
	}
	plan, err := optional(filepath.Join(dir, "plan.json"))
	if err != nil {
		return nil, err
	}
	mapping, err := concerns.Load(filepath.Join(artifacts, "criteria", text(target["standards"])))
	if err != nil {
		return nil, err
	}
	result := summary(mapping, c, changes, plan)
	decisions := docs(result["decisions"])
	changed := clone(target)
	changed["files"] = changes.Files
	changed["changes"] = decisions
	changed["unlisted"] = result["unlisted"]
	if err = save(filepath.Join(dir, "changes.json"), changed); err != nil {
		return nil, err
	}
	for _, q := range qualities() {
		material, err := read(filepath.Join(dir, "materials", q+".json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		own := []doc{}
		rows := []doc{}
		for _, d := range decisions {
			if q == "scope" || q == "readability" || d["quality"] == q {
				own = append(own, d)
			}
		}
		for _, r := range docs(plan["decisions"]) {
			if r["quality"] == q {
				rows = append(rows, r)
			}
		}
		files := []gitx.File{}
		for _, f := range changes.Files {
			include := q == "scope" || q == "readability" || q == "structure" && structural(f.Status)
			for _, p := range []string{f.Path, f.OldPath} {
				if p == "" {
					continue
				}
				if config.Matches(c.Qualities[q].Paths, p) {
					include = true
				}
				for _, d := range own {
					if covers(text(d["path"]), p) {
						include = true
					}
				}
				for _, r := range rows {
					for _, touch := range stringsOf(r["touches"]) {
						if covers(touch, p) {
							include = true
						}
					}
				}
			}
			if include {
				files = append(files, f)
			}
		}
		material["files"] = files
		material["changes"] = own
		material["rows"] = rows
		material["without_plan"] = []doc{}
		if plan == nil {
			material["without_plan"] = own
		}
		if q == "structure" {
			facts, err := structureFacts(g, text(target["base"]), text(target["commit"]), changes, plan)
			if err != nil {
				return nil, err
			}
			material["facts"] = facts
		}
		if err = save(filepath.Join(dir, "materials", q+".json"), material); err != nil {
			return nil, err
		}
	}
	for _, f := range changes.Files {
		old := f.OldPath
		if old == "" {
			old = f.Path
		}
		for _, side := range []struct{ name, ref, path string }{{"base", text(target["base"]), old}, {"commit", text(target["commit"]), f.Path}} {
			ref := side.ref + ":" + side.path
			if !g.OK("cat-file", "-e", ref) {
				continue
			}
			file := filepath.Join(dir, "source", side.name, side.path)
			if err = os.MkdirAll(filepath.Dir(file), 0755); err != nil {
				return nil, err
			}
			source, err := g.Bytes("show", ref)
			if err != nil {
				return nil, err
			}
			if err = os.WriteFile(file, source, 0644); err != nil {
				return nil, err
			}
		}
	}
	delete(result, "decisions")
	delete(result, "unlisted")
	return result, nil
}

func StructureFacts(g *gitx.Git, base, commit string, changes gitx.Changes, plan doc) (doc, error) {
	return structureFacts(g, base, commit, changes, plan)
}

func routeChanges(root, artifacts string, routed doc) (doc, error) {
	dir := filepath.Join(artifacts, "judge")
	target, err := read(filepath.Join(dir, "target.json"))
	if err != nil {
		return nil, err
	}
	c, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	g, err := gitx.New(root, text(target["base_branch"]))
	if err != nil {
		return nil, err
	}
	changes, err := g.Changes(text(target["base"]), text(target["commit"]))
	if err != nil {
		return nil, err
	}
	plan, err := optional(filepath.Join(dir, "plan.json"))
	if err != nil {
		return nil, err
	}
	mapping, err := concerns.Load(filepath.Join(artifacts, "criteria", text(target["standards"])))
	if err != nil {
		return nil, err
	}
	launch := summary(mapping, c, changes, plan)
	for _, item := range docs(routed["qualities"]) {
		q, file := text(item["quality"]), text(item["file"])
		if !slices.Contains(mapping.Judged("code"), q) || text(item["reason"]) == "" {
			return nil, fmt.Errorf("invalid route quality or reason: %s", q)
		}
		if !slices.ContainsFunc(changes.Files, func(f gitx.File) bool { return f.Path == file || f.OldPath == file }) {
			return nil, fmt.Errorf("route file is not changed: %s", file)
		}
		launch["launch_"+q] = true
		if q == "test" || q == "readability" {
			launch["launch_test"] = true
			launch["launch_readability"] = true
		}
		material, err := read(filepath.Join(dir, "materials", q+".json"))
		if err != nil {
			return nil, err
		}
		material["routes"] = append(docs(material["routes"]), item)
		files := docs(material["files"])
		if !slices.ContainsFunc(files, func(f doc) bool { return f["path"] == file || f["oldPath"] == file }) {
			for _, f := range changes.Files {
				if f.Path == file || f.OldPath == file {
					b, _ := json.Marshal(f)
					var d doc
					if err = json.Unmarshal(b, &d); err != nil {
						return nil, err
					}
					files = append(files, d)
				}
			}
		}
		material["files"] = files
		if err = save(filepath.Join(dir, "materials", q+".json"), material); err != nil {
			return nil, err
		}
	}
	if err = save(filepath.Join(dir, "route.json"), routed); err != nil {
		return nil, err
	}
	delete(launch, "decisions")
	delete(launch, "unlisted")
	return launch, nil
}
