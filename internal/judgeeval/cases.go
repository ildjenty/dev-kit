package judgeeval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type document = map[string]any

type Finding struct {
	ID      string `json:"id"`
	Quality string `json:"quality"`
	File    string `json:"file"`
	Summary string `json:"summary"`
}

type Case struct {
	Description  string    `json:"description"`
	Expected     []Finding `json:"expected"`
	NotExpected  []Finding `json:"not_expected,omitempty"`
	Requirements string    `json:"requirements,omitempty"`
	Name         string    `json:"-"`
	Dir          string    `json:"-"`
}

func strictObject(properties document) document {
	keys := []string{}
	for key := range properties {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return document{"type": "object", "additionalProperties": false, "properties": properties, "required": keys}
}

func validate(schema document, value any) error {
	data, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	var source any
	if err = json.Unmarshal(data, &source); err != nil {
		return err
	}
	data, err = json.Marshal(value)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &value); err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	const url = "https://devkit.invalid/eval.json"
	if err := compiler.AddResource(url, source); err != nil {
		return err
	}
	compiled, err := compiler.Compile(url)
	if err != nil {
		return err
	}
	return compiled.Validate(value)
}

func caseSchema() document {
	text := document{"type": "string", "pattern": `\S`}
	qualities := []string{}
	for _, q := range concerns.Names() {
		if q != "delivery" {
			qualities = append(qualities, q)
		}
	}
	finding := strictObject(document{"id": text, "quality": document{"type": "string", "enum": qualities}, "file": text, "summary": text})
	list := document{"type": "array", "items": finding}
	schema := strictObject(document{"description": text, "expected": list, "not_expected": list, "requirements": text})
	schema["required"] = []string{"description", "expected"}
	return schema
}

func repositoryPath(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.Contains(name, "\\") && !slices.Contains(strings.Split(name, "/"), ".git")
}

func loadCase(dir string) (Case, error) {
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return Case{}, fmt.Errorf("%s: case must be a directory", dir)
	}
	file := filepath.Join(dir, "case.json")
	if info, err := os.Lstat(file); err != nil || !info.Mode().IsRegular() {
		return Case{}, fmt.Errorf("%s: case.json must be a regular file", dir)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return Case{}, err
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		return Case{}, fmt.Errorf("%s: %w", file, err)
	}
	if err = validate(caseSchema(), value); err != nil {
		return Case{}, fmt.Errorf("%s: %w", file, err)
	}
	var c Case
	if err = json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	c.Dir, c.Name = dir, filepath.Base(dir)
	ids := map[string]bool{}
	for _, item := range append(append([]Finding{}, c.Expected...), c.NotExpected...) {
		if ids[item.ID] {
			return Case{}, fmt.Errorf("%s: duplicate finding id %q", file, item.ID)
		}
		ids[item.ID] = true
		if !repositoryPath(item.File) {
			return Case{}, fmt.Errorf("%s: finding %q file must be a repository-relative file", file, item.ID)
		}
	}
	base := filepath.Join(dir, "base")
	if info, err := os.Lstat(base); err != nil || !info.IsDir() {
		return Case{}, fmt.Errorf("%s: base must be a directory", dir)
	}
	if err = fs.WalkDir(os.DirFS(base), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name != "." && (!repositoryPath(name) || !entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("%s: base/%s must be a regular repository file or directory", dir, name)
		}
		return nil
	}); err != nil {
		return Case{}, err
	}
	patch := filepath.Join(dir, "change.patch")
	info, err := os.Lstat(patch)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return Case{}, fmt.Errorf("%s: change.patch must be a non-empty regular file", dir)
	}
	return c, nil
}

func loadCases(dir string, names []string) ([]Case, error) {
	if len(names) == 0 {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no judge cases in %s", dir)
	}
	cases := []Case{}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "." || !fs.ValidPath(name) || path.Base(name) != name || strings.Contains(name, "\\") {
			return nil, fmt.Errorf("invalid case name %q: expected one folder name", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate case name %q", name)
		}
		seen[name] = true
		c, err := loadCase(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}
