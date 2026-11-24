package judgeeval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func inputSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	testutil.OK(t, fs.WalkDir(os.DirFS(root), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == ".git" {
			return fs.SkipDir
		}
		if !entry.IsDir() {
			files[name] = string(testutil.Must(os.ReadFile(filepath.Join(root, name))))
		}
		return nil
	}))
	return files
}

func TestEvalRejectsAllInvalidCasesBeforeProviderCalls(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, explicit := range []bool{false, true} {
			for _, tc := range []struct{ name, body, patch, want string }{
				{"bad patch", validCase, "not a patch", "apply --index"},
				{"inapplicable patch", validCase, strings.ReplaceAll(validPatch, "return 1", "return 99"), "apply --index"},
				{"missing expected file", strings.ReplaceAll(validCase, "src/change.py", "missing.py"), validPatch, "absent from base and change"},
				{"missing not expected file", `{"description":"x","expected":[],"not_expected":[{"id":"x","quality":"data","file":"missing.py","summary":"absent"}]}`, validPatch, "absent from base and change"},
				{"directory finding", strings.ReplaceAll(validCase, "src/change.py", "src"), validPatch, "is not a file"},
			} {
				t.Run(fmt.Sprintf("%s/explicit=%v/%s", provider, explicit, tc.name), func(t *testing.T) {
					root := testutil.Repo(t)
					cases := filepath.Join(root, "cases")
					putCase(t, cases, "a-valid", validCase)
					invalid := putCase(t, cases, "z-invalid", tc.body)
					testutil.Put(t, invalid, "change.patch", tc.patch)
					testutil.Put(t, root, ".devkit/config.json", fmt.Sprintf(`{"provider":%q}`, provider))
					env, _, stderr, calls := fakeEnv(t, root, false)
					out := t.TempDir()
					testutil.Put(t, out, "result.json", "previous result\n")
					before := inputSnapshot(t, root)
					head := testutil.Git(t, root, "rev-parse", "HEAD")
					status := testutil.Git(t, root, "status", "--porcelain")
					args := []string{"eval", "judge", "--dir", "cases", "--out", out}
					if explicit {
						args = append(args, "--case", "a-valid", "--case", "z-invalid")
					}
					testutil.Equal(t, cli.New("test", Commands()).Execute(args, env), 1)
					testutil.Contains(t, stderr.String(), "z-invalid")
					testutil.Contains(t, stderr.String(), tc.want)
					_, err := os.Stat(calls)
					testutil.Equal(t, os.IsNotExist(err), true)
					testutil.Equal(t, inputSnapshot(t, out), map[string]string{"result.json": "previous result\n"})
					testutil.Equal(t, inputSnapshot(t, root), before)
					testutil.Equal(t, testutil.Git(t, root, "rev-parse", "HEAD"), head)
					testutil.Equal(t, testutil.Git(t, root, "status", "--porcelain"), status)
				})
			}
		}
	}
}

func TestEvalSemanticErrorsAreNotScores(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, tc := range []struct{ mode, quality, want string }{
			{"empty criteria", "scope", "empty criteria"},
			{"missing aspect", "scope", "Missing required criterion"},
			{"invalid deferred", "scope", "Invalid deferred finding"},
			{"structure blocking", "structure", "Invalid deferred finding"},
			{"structure decision", "structure", "Invalid deferred finding"},
		} {
			t.Run(provider+"/"+tc.mode, func(t *testing.T) {
				root := testutil.Repo(t)
				installTestPack(t, root)
				integrationCase(t, filepath.Join(root, "cases"), false)
				testutil.Put(t, root, ".devkit/config.json", fmt.Sprintf(`{"provider":%q,"language":"fr"}`, provider))
				env, stdout, _, calls := fakeEnv(t, root, false)
				env = withEnv(env, map[string]string{"DEVKIT_EVAL_SEMANTIC": tc.mode})
				out := t.TempDir()
				testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"eval", "judge", "--dir", "cases", "--out", out}, env), 0)
				testutil.Contains(t, stdout.String(), "ERROR (not scored)")
				testutil.Contains(t, stdout.String(), tc.want)
				var result Result
				testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(out, "result.json"))), &result))
				testutil.Equal(t, result.Errored, []string{"sample"})
				testutil.Equal(t, result.Totals, Counts{})
				testutil.Equal(t, result.Qualities, []QualityTotal{})
				item := result.Cases[0]
				testutil.Contains(t, item.Error, tc.want)
				testutil.Equal(t, item.Totals, Counts{})
				var answer document
				testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(item.Artifacts, "judge", tc.quality+"-output.json"))), &answer))
				formats := testutil.Must(schemas(filepath.Join(root, ".archon/workflows/dev"), []string{tc.quality}))
				var schema document
				testutil.OK(t, json.Unmarshal([]byte(formats[tc.quality]), &schema))
				testutil.OK(t, validate(schema, answer))
				if tc.mode == "empty criteria" {
					testutil.Equal(t, len(answer["criteria"].([]any)), 0)
				} else if tc.mode == "missing aspect" {
					testutil.Equal(t, answer["criteria"].([]any)[0].(map[string]any)["aspect"], "other")
				} else {
					testutil.Equal(t, answer["findings"].([]any)[0].(map[string]any)["deferred"], true)
				}
				for _, line := range strings.Split(strings.TrimSpace(string(testutil.Must(os.ReadFile(calls)))), "\n") {
					var call document
					testutil.OK(t, json.Unmarshal([]byte(line), &call))
					if call["kind"] == "matching" {
						t.Fatal("invalid answer was sent to matching")
					}
				}
			})
		}
	}
}
