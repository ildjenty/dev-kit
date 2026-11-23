package main

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/kit"
	"gopkg.in/yaml.v3"
)

func TestWorkflowCommandsAreRegistered(t *testing.T) {
	calls, err := kitinfo.Invocations(kit.Files, ".archon/workflows", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 {
		t.Fatal("no workflow plumbing checked")
	}
	skills, err := kitinfo.Invocations(kit.Files, ".claude/skills", true)
	if err != nil {
		t.Fatal(err)
	}
	calls = append(calls, skills...)
	if missing := newRegistry().MissingCommands(calls); len(missing) > 0 {
		t.Fatalf("unregistered kit commands: %v", missing)
	}
}

func TestFixtureHelpersAreNotShipped(t *testing.T) {
	for _, command := range newRegistry().Commands() {
		if command.Area == "fixture" {
			t.Fatal("fixture helper registered in shipped binary")
		}
	}
}

func TestWorkflowAIContracts(t *testing.T) {
	judges := map[string]bool{"judge": true, "judge-quality": true, "judge-delivery": true, "judge-decide": true, "judge-document": true, "quality": true, "route": true, "reflect": true, "step-back": true, "dispute": true}
	checked := 0
	err := fs.WalkDir(kit.Files, ".archon/workflows", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) || strings.Contains(path, "/fixtures/") {
			return nil
		}
		data, err := kit.Files.ReadFile(path)
		if err != nil {
			return err
		}
		var workflow map[string]any
		if err = yaml.Unmarshal(data, &workflow); err != nil {
			return err
		}
		var visit func(any)
		visit = func(value any) {
			switch value := value.(type) {
			case map[string]any:
				if _, ok := value["wait"]; ok {
					t.Errorf("%s uses a wait node", path)
				}
				if _, ok := value["script"]; ok {
					t.Errorf("%s uses a script node", path)
				}
				if provider, ok := value["provider"]; ok {
					checked++
					if provider != "claude" {
						t.Errorf("%s node %v uses provider %v", path, value["id"], provider)
					}
					command, _ := value["command"].(string)
					if judges[command] {
						if value["context"] != "fresh" || value["mutates_checkout"] != false {
							t.Errorf("%s judge %v is not fresh and read-only", path, value["id"])
						}
						allowed, hasAllowed := value["allowed_tools"].([]any)
						denied, hasDenied := value["denied_tools"].([]any)
						if hasAllowed {
							for _, tool := range allowed {
								if tool != "Read" && tool != "Grep" && tool != "Glob" {
									t.Errorf("%s judge %v allows %v", path, value["id"], tool)
								}
							}
						} else if !hasDenied || !slices.Contains(denied, any("Edit")) || !slices.Contains(denied, any("Write")) || !slices.Contains(denied, any("NotebookEdit")) {
							t.Errorf("%s judge %v lacks read-only tool restrictions", path, value["id"])
						}
					}
				}
				for _, child := range value {
					visit(child)
				}
			case []any:
				for _, child := range value {
					visit(child)
				}
			}
		}
		visit(workflow)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no AI nodes checked")
	}
}

func strictSchema(t *testing.T, path string, value any) {
	t.Helper()
	switch node := value.(type) {
	case map[string]any:
		if properties, ok := node["properties"].(map[string]any); ok {
			required, _ := node["required"].([]any)
			if node["additionalProperties"] != false {
				t.Errorf("%s: object allows additional properties", path)
			}
			for key := range properties {
				if !slices.Contains(required, any(key)) {
					t.Errorf("%s: %s is not required", path, key)
				}
			}
		}
		for key, child := range node {
			strictSchema(t, path+"/"+key, child)
		}
	case []any:
		for _, child := range node {
			strictSchema(t, path, child)
		}
	}
}

func TestEveryAISchemaIsStrict(t *testing.T) {
	count := 0
	err := fs.WalkDir(kit.Files, ".archon/workflows", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".yaml") || strings.Contains(path, "/fixtures/") {
			return err
		}
		data, err := kit.Files.ReadFile(path)
		if err != nil {
			return err
		}
		var root any
		if err := yaml.Unmarshal(data, &root); err != nil {
			return err
		}
		var visit func(any)
		visit = func(value any) {
			switch node := value.(type) {
			case map[string]any:
				if _, ai := node["provider"]; ai {
					count++
					if node["output_format"] == nil {
						t.Errorf("%s node %v lacks a schema", path, node["id"])
					}
					strictSchema(t, path, node["output_format"])
				}
				for _, child := range node {
					visit(child)
				}
			case []any:
				for _, child := range node {
					visit(child)
				}
			}
		}
		visit(root)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no AI schemas checked")
	}
}
