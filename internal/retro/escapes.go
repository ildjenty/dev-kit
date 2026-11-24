package retro

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/judgeeval"
	"github.com/ildjenty/dev-kit/internal/record"
)

func caseFolder(input doc) string { return str(obj(input["folders"])["eval"]) + "/judge" }

func caseName(name string) bool {
	return safePath(name) && name != "." && !strings.Contains(name, "/") && name != ".git"
}

func trackedCase(ctx record.ContextData, base, dir string) (bool, error) {
	out, e := gitx.Run(ctx.Cwd, "ls-tree", "--name-only", base, "--", dir)
	return out != "", e
}

func escapeDraftErrors(ctx record.ContextData, input, draft doc, used map[string]bool) ([]string, error) {
	errors := []string{}
	paths, e := proposalChanges(ctx, input)
	if e != nil {
		return nil, e
	}
	escapes, traces, cases, names := map[string]bool{}, map[string]doc{}, map[string]bool{}, map[string]bool{}
	for _, entry := range items(input["entries"]) {
		if entry["kind"] == "escape" {
			escapes[str(entry["id"])] = true
		}
	}
	for _, trace := range items(draft["escapes"]) {
		id := str(trace["entry"])
		if !escapes[id] || traces[id] != nil {
			errors = append(errors, "escape trace must name one distinct escape: "+id)
		}
		traces[id] = trace
		if trace["judge_should_catch"] == true && trace["quality"] == "" {
			errors = append(errors, "judge escape requires a quality: "+id)
		}
		if (trace["stage"] == "routing" || trace["stage"] == "quality") && (trace["judge_should_catch"] != true || trace["quality"] == "") {
			errors = append(errors, "judge or routing escape requires a quality and judge_should_catch: "+id)
		}
		if trace["stage"] == "none" && trace["judge_should_catch"] == true {
			errors = append(errors, "a genuine request gap cannot require a judge case: "+id)
		}
		if trace["stage"] == "routing" && used[id] {
			found := false
			for _, edit := range items(draft["standards"]) {
				if str(edit["path"]) == str(obj(input["folders"])["standards"])+"/concerns.json" {
					for _, ref := range stringsOf(edit["entries"]) {
						found = found || ref == id
					}
				}
			}
			if !found {
				errors = append(errors, "routing escape requires a traced concerns.json edit: "+id)
			}
			current, e := concerns.Load(filepath.Join(ctx.Cwd, str(obj(input["folders"])["standards"])))
			if e == nil {
				quality := str(trace["quality"])
				original := obj(obj(obj(input["concerns"])["qualities"])[quality])
				if current.Qualities[quality].Routing == str(original["routing"]) {
					errors = append(errors, "routing escape requires a changed routing sentence: "+id)
				}
			}
		}
	}
	settings, e := config.Load(ctx.Cwd)
	if e != nil {
		return append(errors, e.Error()), nil
	}
	for _, item := range items(draft["cases"]) {
		id, name := str(item["entry"]), str(item["name"])
		if !escapes[id] || !used[id] || traces[id]["judge_should_catch"] != true || cases[id] {
			errors = append(errors, "evaluation case must name one used escape the judge should catch: "+id)
		}
		cases[id] = true
		if !caseName(name) || names[name] {
			errors = append(errors, "invalid or duplicate evaluation case name: "+name)
			continue
		}
		names[name] = true
		dir := caseFolder(input) + "/" + name
		baseFile := false
		for _, path := range paths {
			baseFile = baseFile || inside(path, dir+"/base")
		}
		if !baseFile {
			errors = append(errors, "escape case base must contain a file to survive Git commits (use .gitkeep for an empty base): "+name)
		}
		tracked, e := trackedCase(ctx, str(input["proposal_base"]), dir)
		if e != nil {
			return nil, e
		}
		if tracked {
			errors = append(errors, "evaluation case already exists: "+dir)
		}
		ignored, e := gitx.Run(ctx.Cwd, "ls-files", "--others", "--ignored", "--exclude-standard", "--", dir)
		if e != nil {
			return nil, e
		}
		if ignored != "" {
			errors = append(errors, "evaluation case files must not be ignored by Git: "+ignored)
		}
		c, e := judgeeval.ValidateCase(filepath.Join(ctx.Cwd, dir), settings, os.DirFS(filepath.Join(ctx.Cwd, settings.Standards)))
		if e != nil {
			errors = append(errors, fmt.Sprintf("evaluation case %s: %s", name, e))
		} else if len(c.Expected) == 0 {
			errors = append(errors, "escape evaluation case must have an expected finding: "+name)
		} else {
			found := false
			for _, finding := range c.Expected {
				found = found || finding.Quality == traces[id]["quality"]
			}
			if !found {
				errors = append(errors, "escape case must expect the traced quality: "+name)
			}
		}
	}
	for id := range escapes {
		if traces[id] == nil {
			errors = append(errors, "escape requires a stage trace: "+id)
		}
		if used[id] && traces[id]["judge_should_catch"] == true && !cases[id] {
			errors = append(errors, "used escape requires an evaluation case: "+id)
		}
	}
	for _, path := range paths {
		if inside(path, caseFolder(input)) {
			rel := strings.TrimPrefix(path, caseFolder(input)+"/")
			name, file, _ := strings.Cut(rel, "/")
			if !names[name] {
				errors = append(errors, "evaluation case edit has no escape trace: "+path)
			} else if file != "case.json" && file != "change.patch" && !inside(file, "base") {
				errors = append(errors, "evaluation case file must be case.json, change.patch or under base/: "+path)
			}
		}
	}
	return errors, nil
}
