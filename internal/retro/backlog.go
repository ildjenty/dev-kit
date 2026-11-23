package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		for _, kind := range []string{"deferred", "carried", "mechanical", "accepted_objections"} {
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
				if str(finding["id"]) == "" || str(source["judge"]) == "" || str(source["run_id"]) == "" || num(source["judgment"]) < 1 || str(source["kind"]) == "" || source["target"] == nil {
					return nil, nil, fmt.Errorf("%s: %s/%d requires finding and source judgment", rel, kind, i)
				}
				entries = append(entries, doc{"id": fmt.Sprintf("%s#%s/%d", rel, kind, i), "path": rel, "kind": kind, "content": item})
			}
		}
	}
	return entries, files, nil
}
