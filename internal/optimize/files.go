package optimize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type doc = map[string]any

var aspects = []string{"fit", "harness", "triage"}

func obj(v any) doc    { m, _ := v.(map[string]any); return m }
func str(v any) string { s, _ := v.(string); return s }
func num(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}
func array(v any) []any {
	switch a := v.(type) {
	case []any:
		return a
	case []doc:
		r := []any{}
		for _, d := range a {
			r = append(r, d)
		}
		return r
	case []string:
		r := []any{}
		for _, s := range a {
			r = append(r, s)
		}
		return r
	}
	return []any{}
}
func items(v any) []doc {
	r := []doc{}
	for _, v := range array(v) {
		r = append(r, obj(v))
	}
	return r
}
func stringsOf(v any) []string {
	r := []string{}
	for _, s := range array(v) {
		r = append(r, str(s))
	}
	return r
}
func readJSON(path string) (doc, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var v doc
	e = json.Unmarshal(b, &v)
	return v, e
}
func readDraft(artifacts string) (any, error) {
	b, e := os.ReadFile(file(artifacts, "draft.json"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var d any
	if e := json.Unmarshal(b, &d); e != nil {
		return nil, fmt.Errorf("%s: %w", file(artifacts, "draft.json"), e)
	}
	return d, nil
}
func put(path, text string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, []byte(text), 0644)
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return put(path, string(b)+"\n")
}
func resolve(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}

func file(artifacts, name string) string { return filepath.Join(artifacts, "optimize", name) }
