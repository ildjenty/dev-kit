package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func inside(path, dir string) bool { return strings.HasPrefix(path, dir+"/") }
func safePath(path string) bool {
	return path != "" && !filepath.IsAbs(path) && filepath.ToSlash(filepath.Clean(path)) == path && path != ".." && !strings.HasPrefix(path, "../") && !strings.Contains(path, "\\")
}
func changes(cwd string) ([]string, error) {
	paths := []string{}
	for _, args := range [][]string{{"diff", "--cached", "--name-only", "--no-renames", "-z"}, {"diff", "--name-only", "--no-renames", "-z"}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		out, e := gitx.Run(cwd, args...)
		if e != nil {
			return nil, e
		}
		for _, p := range strings.Split(out, "\x00") {
			if p != "" && !slices.Contains(paths, p) {
				paths = append(paths, p)
			}
		}
	}
	slices.Sort(paths)
	return paths, nil
}
func scopeErrors(ctx record.ContextData, input doc, backlog bool) ([]string, error) {
	paths, e := changes(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	folders := obj(input["folders"])
	errors := []string{}
	for _, p := range paths {
		allowed := safePath(p) && (inside(p, str(folders["units"])) || inside(p, str(folders["standards"])) || p == ".devkit/config.json")
		if backlog && obj(input["backlog"])[p] != nil {
			allowed = true
		}
		if !allowed {
			errors = append(errors, "edited outside the allowed paths: "+p)
		}
	}
	return errors, nil
}
func draftErrors(ctx record.ContextData, artifacts string, value any) ([]string, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	errors, e := schema("draft", value)
	if e != nil || len(errors) > 0 {
		return errors, e
	}
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	baseline, e := readJSON(file(artifacts, "baseline.json"))
	if e != nil {
		return nil, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	if head != baseline["commit"] {
		errors = append(errors, "writer changed HEAD; only finish may commit")
	}
	draft := obj(value)
	known, used, dismissed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, entry := range items(input["entries"]) {
		known[str(entry["id"])] = true
	}
	for _, kind := range []string{"units", "checks", "standards", "dismissed"} {
		seen := map[string]bool{}
		key := map[string]string{"units": "key", "checks": "name", "standards": "path"}[kind]
		for _, item := range items(draft[kind]) {
			if key != "" {
				k := str(item[key])
				if seen[k] {
					errors = append(errors, "duplicate "+key+": "+k)
				}
				seen[k] = true
			}
			for _, id := range stringsOf(item["entries"]) {
				if !known[id] {
					errors = append(errors, "unknown backlog entry: "+id)
				}
				if kind == "dismissed" {
					dismissed[id] = true
				} else {
					used[id] = true
				}
			}
		}
	}
	for _, entry := range items(input["entries"]) {
		id := str(entry["id"])
		if !used[id] && !dismissed[id] {
			errors = append(errors, "backlog entry is neither used nor dismissed: "+id)
		}
		if used[id] && dismissed[id] {
			errors = append(errors, "backlog entry is both used and dismissed: "+id)
		}
	}
	se, e := scopeErrors(ctx, input, false)
	if e != nil {
		return nil, e
	}
	errors = append(errors, se...)
	paths, e := changes(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	staged, e := gitx.Run(ctx.Cwd, "diff", "--cached", "--name-only", "--no-renames", "-z")
	if e != nil {
		return nil, e
	}
	unstaged, e := gitx.Run(ctx.Cwd, "diff", "--name-only", "--no-renames", "-z")
	if e != nil {
		return nil, e
	}
	for _, p := range strings.Split(staged, "\x00") {
		if p != "" && slices.Contains(strings.Split(unstaged, "\x00"), p) {
			errors = append(errors, "index and worktree disagree: "+p)
		}
	}
	standards := str(obj(input["folders"])["standards"])
	named := []string{}
	for _, n := range items(draft["standards"]) {
		p := str(n["path"])
		if !safePath(p) || !inside(p, standards) {
			errors = append(errors, "standard edit is outside the standards folder: "+p)
		}
		if !slices.Contains(paths, p) {
			errors = append(errors, "standard edit has no checkout change: "+p)
		}
		named = append(named, p)
	}
	for _, p := range paths {
		if inside(p, standards) && !slices.Contains(named, p) {
			errors = append(errors, "standard edit has no backlog trace: "+p)
		}
		if inside(p, str(obj(input["folders"])["units"])) {
			errors = append(errors, "write units in the draft, not directly: "+p)
		}
	}
	current, e := config.Load(ctx.Cwd)
	if e != nil {
		errors = append(errors, e.Error())
	} else {
		b, e := json.Marshal(input["config"])
		if e != nil {
			return nil, e
		}
		var expected config.Config
		if e = json.Unmarshal(b, &expected); e != nil {
			return nil, e
		}
		for _, item := range items(draft["checks"]) {
			b, e = json.Marshal(doc{"name": item["name"], "command": item["command"], "paths": item["paths"], "timeout_seconds": item["timeout_seconds"]})
			if e != nil {
				return nil, e
			}
			var check config.Check
			if e = json.Unmarshal(b, &check); e != nil {
				return nil, e
			}
			expected.Checks = append(expected.Checks, check)
		}
		if !reflect.DeepEqual(expected, current) {
			errors = append(errors, "config must contain exactly the proposed check additions and preserve other settings")
		}
	}
	if _, e = concerns.Load(filepath.Join(ctx.Cwd, standards)); e != nil {
		errors = append(errors, e.Error())
	}
	for _, p := range paths {
		info, e := os.Lstat(filepath.Join(ctx.Cwd, p))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() {
			errors = append(errors, "edited path must be a regular file: "+p)
		}
	}
	b, e := os.ReadFile(file(artifacts, "approval.txt"))
	if os.IsNotExist(e) || e == nil && strings.TrimSpace(string(b)) == "" {
		errors = append(errors, "approval.txt is required in the configured language")
	} else if e != nil {
		return nil, e
	} else if !strings.Contains(string(b), fmt.Sprintf("approve=%d", num(input["next_proposal_version"]))) || !strings.Contains(string(b), "answers") {
		errors = append(errors, "approval.txt must explain approve=<version> and answers")
	}
	return errors, nil
}
func check(ctx record.ContextData, artifacts string) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	draft, e := readDraft(artifacts)
	if e != nil {
		return nil, e
	}
	errors, e := draftErrors(ctx, artifacts, draft)
	if e != nil {
		return nil, e
	}
	out := doc{"valid": len(errors) == 0, "errors": errors}
	return out, writeJSON(file(artifacts, "check.json"), out)
}
