package main

import (
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/kit"
	"gopkg.in/yaml.v3"
)

func TestWorkflowCommandsAreRegistered(t *testing.T) {
	commands := map[string]bool{}
	areas := map[string]bool{}
	for _, command := range newRegistry().Commands() {
		if command.Name == "" {
			areas[command.Area] = true
		} else {
			commands[command.Area+" "+command.Name] = true
		}
	}
	for _, command := range newRegistry().Commands() {
		if command.Name != "" {
			delete(areas, command.Area)
		}
	}
	call := regexp.MustCompile(`\bdevkit[ \t]+([a-z][a-z-]*)(?:[ \t]+([a-z][a-z-]*))?`)
	files, calls := 0, 0
	err := fs.WalkDir(kit.Files, ".archon/workflows", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") || strings.Contains(path, "/commands/") && strings.HasSuffix(path, ".md")) {
			return nil
		}
		files++
		data, err := kit.Files.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range call.FindAllStringSubmatch(string(data), -1) {
			calls++
			if !areas[match[1]] && !commands[match[1]+" "+match[2]] {
				t.Errorf("%s calls unregistered command %q", path, match[0])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 || calls == 0 {
		t.Fatalf("no workflow plumbing checked: %d files, %d calls", files, calls)
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
