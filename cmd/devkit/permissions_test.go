package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func TestInstalledJudgePermissions(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			root := t.TempDir()
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: root, Stdout: &out, Stderr: &stderr}
			testutil.Equal(t, newRegistry().Execute([]string{"init", "--provider", provider}, env), 0)
			checked := 0
			for workflow, ids := range map[string][]string{
				"judge":     {"judge-data", "judge-contract", "judge-security", "judge-ui", "judge-cost", "judge-test", "judge-scope", "judge-readability"},
				"implement": {"step-back"},
				"fix":       {"dispute", "step-back"},
			} {
				path := filepath.Join(root, ".archon/workflows/dev", workflow, workflow+".yaml")
				var document any
				testutil.OK(t, yaml.Unmarshal(testutil.Must(os.ReadFile(path)), &document))
				var visit func(any)
				visit = func(value any) {
					switch node := value.(type) {
					case map[string]any:
						id, _ := node["id"].(string)
						if slices.Contains(ids, id) {
							checked++
							testutil.Equal(t, node["provider"], provider)
							testutil.Equal(t, node["context"], "fresh")
							testutil.Equal(t, node["mutates_checkout"], false)
							if provider == "claude" {
								testutil.Equal(t, node["allowed_tools"], []string{"Read", "Grep", "Glob"})
							} else {
								for _, key := range []string{"allowed_tools", "denied_tools", "settingSources"} {
									if _, ok := node[key]; ok {
										t.Errorf("%s node %s retained %s", workflow, id, key)
									}
								}
							}
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
				visit(document)
			}
			testutil.Equal(t, checked, 11)
			testutil.Equal(t, stderr.String(), "")
		})
	}
}
