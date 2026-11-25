package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	backlogschema "github.com/ildjenty/dev-kit/internal/backlog"
	"github.com/ildjenty/dev-kit/internal/escape"
)

func readBacklog(cwd string) ([]doc, doc, error) {
	paths, e := backlogPaths(filepath.Join(cwd, ".devkit/backlog"))
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
		entriesOnly := doc{}
		for _, kind := range []string{"deferred", "carried", "mechanical", "accepted_objections", "review"} {
			if v, exists := value[kind]; exists {
				entriesOnly[kind] = v
			}
		}
		entryData, e := json.Marshal(entriesOnly)
		if e != nil {
			return nil, nil, e
		}
		if e = backlogschema.Validate(entryData); e != nil {
			return nil, nil, fmt.Errorf("%s: %w", rel, e)
		}
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
				entries = append(entries, doc{"id": fmt.Sprintf("%s#%s/%d", rel, kind, i), "path": rel, "kind": kind, "content": item})
			}
		}
	}
	paths, e = backlogPaths(filepath.Join(cwd, ".devkit/backlog/escapes"))
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

func backlogPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	return paths, nil
}
