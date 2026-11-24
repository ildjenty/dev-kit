package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/escape"
)

func readBacklog(cwd string) ([]doc, doc, error) {
	paths, e := filepath.Glob(filepath.Join(cwd, ".devkit/backlog/*.json"))
	if e != nil {
		return nil, nil, e
	}
	entries, files := []doc{}, doc{}
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			return nil, nil, e
		}
		var value doc
		if e = json.Unmarshal(b, &value); e != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, e)
		}

		rel, e := filepath.Rel(cwd, path)
		if e != nil {
			return nil, nil, e
		}
		rel = filepath.ToSlash(rel)
		files[rel] = string(b)
		for _, kind := range []string{"deferred", "carried", "mechanical", "accepted_objections", "review"} {
			if _, exists := value[kind]; kind == "review" && !exists {
				continue
			}
			values, ok := value[kind].([]any)
			if !ok {
				return nil, nil, fmt.Errorf("%s: %s must be an array", rel, kind)
			}
			for i, v := range values {
				item, ok := v.(map[string]any)
				if !ok {
					return nil, nil, fmt.Errorf("%s: %s/%d must be an object", rel, kind, i)
				}
				finding, source := obj(item["finding"]), obj(item["source"])
				if str(finding["id"]) == "" || str(source["run_id"]) == "" || str(source["kind"]) == "" || kind != "review" && (str(source["judge"]) == "" || num(source["judgment"]) < 1) || kind == "review" && (source["kind"] != "review" || str(obj(source["target"])["commit"]) == "") || source["target"] == nil {
					return nil, nil, fmt.Errorf("%s: %s/%d requires finding and source", rel, kind, i)
				}
				entries = append(entries, doc{"id": fmt.Sprintf("%s#%s/%d", rel, kind, i), "path": rel, "kind": kind, "content": item})
			}
		}
	}
	paths, e = filepath.Glob(filepath.Join(cwd, ".devkit/backlog/escapes/*.json"))
	if e != nil {
		return nil, nil, e
	}
	for _, path := range paths {
		if !escape.ID.MatchString(strings.TrimSuffix(filepath.Base(path), ".json")) {
			return nil, nil, fmt.Errorf("%s: invalid escape id", path)
		}
		if info, e := os.Lstat(path); e != nil || !info.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("%s: escape must be a regular file", path)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return nil, nil, e
		}
		if _, e = escape.Validate(b); e != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, e)
		}
		rel, e := filepath.Rel(cwd, path)
		if e != nil {
			return nil, nil, e
		}
		rel = filepath.ToSlash(rel)
		var value doc
		if e = json.Unmarshal(b, &value); e != nil {
			return nil, nil, e
		}
		files[rel] = string(b)
		entries = append(entries, doc{"id": rel + "#escape/0", "path": rel, "kind": "escape", "content": value})
	}
	return entries, files, nil
}
