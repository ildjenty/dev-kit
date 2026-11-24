package judgeeval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
	"gopkg.in/yaml.v3"
)

const validCase = `{"description":"Known change","expected":[{"id":"found","quality":"contract","file":"src/change.py","summary":"Expected marker: failures become successful defaults."}]}`
const validPatch = "--- a/src/change.py\n+++ b/src/change.py\n@@ -1 +1 @@\n-def value(): return 1\n+def value(): return 2\n"

func putCase(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	testutil.Put(t, dir, "case.json", body)
	testutil.Put(t, dir, "base/src/change.py", "def value(): return 1\n")
	testutil.Put(t, dir, "base/src/unchanged.py", "def peer(): return 3\n")
	testutil.Put(t, dir, "change.patch", validPatch)
	return dir
}

func TestCaseValidation(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"syntax", `{`, "case.json"},
		{"null", `null`, "want object"},
		{"missing expected", `{"description":"x"}`, "expected"},
		{"null expected", `{"description":"x","expected":null}`, "want array"},
		{"unknown field", `{"description":"x","expected":[],"extra":true}`, "additional properties"},
		{"blank description", `{"description":"  ","expected":[]}`, "pattern"},
		{"blank requirements", `{"description":"x","expected":[],"requirements":" "}`, "pattern"},
		{"nontext requirements", `{"description":"x","expected":[],"requirements":{}}`, "want string"},
		{"null not expected", `{"description":"x","expected":[],"not_expected":null}`, "want array"},
		{"unknown quality", strings.ReplaceAll(validCase, "contract", "unknown"), "value must be one of"},
		{"plan only quality", strings.ReplaceAll(validCase, "contract", "delivery"), "value must be one of"},
		{"missing finding summary", strings.ReplaceAll(validCase, `,"summary":"Expected marker: failures become successful defaults."`, ""), "summary"},
		{"extra finding property", strings.ReplaceAll(validCase, `"id":"found"`, `"extra":true,"id":"found"`), "additional properties"},
		{"blank summary", strings.ReplaceAll(validCase, "Expected marker: failures become successful defaults.", " "), "pattern"},
		{"parent path", strings.ReplaceAll(validCase, "src/change.py", "../change.py"), "repository-relative"},
		{"absolute path", strings.ReplaceAll(validCase, "src/change.py", "/tmp/change.py"), "repository-relative"},
		{"metadata path", strings.ReplaceAll(validCase, "src/change.py", ".git/config"), "repository-relative"},
		{"duplicate IDs", `{"description":"x","expected":[{"id":"x","quality":"data","file":"a","summary":"a"}],"not_expected":[{"id":"x","quality":"data","file":"a","summary":"b"}]}`, "duplicate finding id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := putCase(t, t.TempDir(), "sample", tc.body)
			_, err := loadCase(dir)
			testutil.Fails(t, err, tc.want)
		})
	}
	for _, tc := range []struct{ name, remove, want string }{{"base missing", "base", "base must be"}, {"patch missing", "change.patch", "change.patch must be"}, {"case missing", "case.json", "case.json must be"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := putCase(t, t.TempDir(), "sample", validCase)
			testutil.OK(t, os.RemoveAll(filepath.Join(dir, tc.remove)))
			_, err := loadCase(dir)
			testutil.Fails(t, err, tc.want)
		})
	}
	t.Run("metadata in base", func(t *testing.T) {
		dir := putCase(t, t.TempDir(), "sample", validCase)
		testutil.Put(t, dir, "base/.git/config", "metadata")
		_, err := loadCase(dir)
		testutil.Fails(t, err, "base/.git")
	})
	t.Run("symlink in base", func(t *testing.T) {
		dir := putCase(t, t.TempDir(), "sample", validCase)
		testutil.OK(t, os.Symlink("change.py", filepath.Join(dir, "base/src/link.py")))
		_, err := loadCase(dir)
		testutil.Fails(t, err, "base/src/link.py")
	})
}

func TestCaseSelection(t *testing.T) {
	root := t.TempDir()
	putCase(t, root, "b", validCase)
	putCase(t, root, "a", `{"description":"Clean","expected":[]}`)
	cases := testutil.Must(loadCases(root, nil))
	testutil.Equal(t, []string{cases[0].Name, cases[1].Name}, []string{"a", "b"})
	testutil.Equal(t, testutil.Must(loadCases(root, []string{"b"}))[0].Name, "b")
	for _, names := range [][]string{{"../b"}, {"."}, {"/a"}, {"a/b"}, {"a", "a"}, {"missing"}} {
		_, err := loadCases(root, names)
		if err == nil {
			t.Fatalf("accepted %v", names)
		}
	}
	_, err := loadCases(t.TempDir(), nil)
	testutil.Fails(t, err, "no judge cases")
}

func embeddedStandards(t *testing.T) fs.FS {
	t.Helper()
	return testutil.Must(fs.Sub(kit.Files, ".devkit/standards"))
}

func TestSeedRepositories(t *testing.T) {
	cases := testutil.Must(loadCases(testutil.Must(filepath.Abs("../../eval/judge")), nil))
	covered := map[string]bool{}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			repo := t.TempDir()
			base := testutil.Must(repository(c, repo, config.Defaults(""), embeddedStandards(t)))
			testutil.Equal(t, base, testutil.Git(t, repo, "rev-parse", "main"))
			testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "--abbrev-ref", "HEAD"), "eval-change")
			testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
			testutil.Equal(t, testutil.Git(t, repo, "rev-list", "--count", "HEAD"), "2")
			for _, f := range c.Expected {
				covered[f.Quality] = true
			}
			if c.Name == "duplicate-price-helper" {
				testutil.Contains(t, testutil.Git(t, repo, "show", "main:src/orders/pricing.ts"), "export function cartTotal")
			}
			if c.Name == "duplicate-customer-converter" {
				testutil.Contains(t, testutil.Git(t, repo, "show", "main:app/src/main/kotlin/acme/customer/CustomerNames.kt"), "fun display")
			}
		})
	}
	for _, q := range []string{"structure", "data", "contract", "security", "ui", "cost", "test", "scope", "readability"} {
		testutil.Equal(t, covered[q], true)
	}
}

func TestRepositoryErrors(t *testing.T) {
	for _, tc := range []struct{ name, body, patch, want string }{
		{"invalid patch", validCase, "not a patch", "apply --index"},
		{"missing finding file", strings.ReplaceAll(validCase, "src/change.py", "missing.py"), validPatch, "absent from base and change"},
		{"directory finding path", strings.ReplaceAll(validCase, "src/change.py", "src"), validPatch, "is not a file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := putCase(t, t.TempDir(), "sample", tc.body)
			testutil.Put(t, dir, "change.patch", tc.patch)
			_, err := repository(testutil.Must(loadCase(dir)), t.TempDir(), config.Defaults(""), embeddedStandards(t))
			testutil.Fails(t, err, tc.want)
		})
	}
}

func integrationCase(t *testing.T, root string, requirements bool) {
	t.Helper()
	var data document
	testutil.OK(t, json.Unmarshal([]byte(validCase), &data))
	data["expected"] = append(data["expected"].([]any), document{"id": "routing", "quality": "security", "file": "src/change.py", "summary": "Expected marker: missing authorization."}, document{"id": "missed", "quality": "test", "file": "src/change.py", "summary": "Expected marker: no failure-path test."})
	data["not_expected"] = []Finding{{ID: "false-positive", Quality: "scope", File: "src/change.py", Summary: "Expected marker: local change is within the request."}}
	if requirements {
		data["requirements"] = "Preserve all other behavior while changing the returned value."
	}
	putCase(t, root, "sample", string(testutil.Must(json.Marshal(data))))
}

func fakeEnv(t *testing.T, root string, requirements bool) (cli.Env, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	fake := filepath.Join(t.TempDir(), "fake-cli")
	exe := testutil.Must(os.Executable())
	testutil.Put(t, filepath.Dir(fake), filepath.Base(fake), "#!/bin/sh\nexec '"+strings.ReplaceAll(exe, "'", "'\\''")+"' -test.run=TestEvalCLIHelper -- \"$@\"\n")
	testutil.OK(t, os.Chmod(fake, 0700))
	calls := filepath.Join(t.TempDir(), "calls.jsonl")
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: root, Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""), Environ: os.Environ(), LookupEnv: os.LookupEnv}
	env = withEnv(env, map[string]string{"DEVKIT_CODEX": fake, "DEVKIT_CLAUDE": fake, "TYPESAFE_API_KEY": "", "DEVKIT_EVAL_FAKE": "1", "DEVKIT_EVAL_CALLS": calls, "DEVKIT_EVAL_REQUIREMENTS": fmt.Sprint(requirements)})
	return env, &stdout, &stderr, calls
}

func installTestPack(t *testing.T, root string) {
	t.Helper()
	pack := filepath.Join(root, ".archon/workflows/dev")
	testutil.OK(t, copyFolder(testutil.Must(fs.Sub(kit.Files, ".archon/workflows/dev")), pack))
	file := filepath.Join(pack, "judge/judge.yaml")
	var data document
	testutil.OK(t, yaml.Unmarshal(testutil.Must(os.ReadFile(file)), &data))
	for _, raw := range data["nodes"].([]any) {
		node := raw.(map[string]any)
		if strings.HasPrefix(node["id"].(string), "judge-") {
			format := node["output_format"].(map[string]any)
			format["properties"].(map[string]any)["eval_probe"] = document{"type": "string"}
			format["required"] = append(format["required"].([]any), "eval_probe")
		}
	}
	testutil.OK(t, os.Chmod(file, 0644))
	testutil.OK(t, os.WriteFile(file, testutil.Must(yaml.Marshal(data)), 0644))
	qualityFile := filepath.Join(pack, "judge/commands/quality.md")
	content := testutil.Must(os.ReadFile(qualityFile))
	testutil.OK(t, os.Chmod(qualityFile, 0644))
	testutil.OK(t, os.WriteFile(qualityFile, append(content, []byte("\nInstalled quality prompt marker.\n")...), 0644))
}

func TestEvalWithFakeProviders(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, requirements := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/requirements=%v", provider, requirements), func(t *testing.T) {
				root := testutil.Repo(t)
				installTestPack(t, root)
				integrationCase(t, filepath.Join(root, "custom-eval/judge"), requirements)
				testutil.Put(t, root, ".devkit/config.json", fmt.Sprintf(`{"provider":%q,"eval":"custom-eval","language":"fr","checks":[{"name":"never-run","command":"exit 99","timeout_seconds":1}]}`, provider))
				env, stdout, stderr, calls := fakeEnv(t, root, requirements)
				env = withEnv(env, map[string]string{"DEVKIT_EVAL_CHECKS": "true"})
				out := filepath.Join(t.TempDir(), "results")
				registry := cli.New("test", Commands())
				code := registry.Execute([]string{"eval", "judge", "--out", out}, env)
				if code != 0 {
					t.Fatalf("exit %d: %s\n%s", code, stdout, stderr)
				}
				var result Result
				testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(out, "result.json"))), &result))
				testutil.Equal(t, result.Totals, Counts{Expected: 3, Found: 1, Missed: 2, RoutingMissed: 1, NotExpected: 1, NotExpectedReported: 1, Other: 1})
				testutil.Contains(t, stdout.String(), "routing: quality not launched")
				testutil.Contains(t, stdout.String(), "NOT-EXPECTED REPORTED")
				testutil.Contains(t, stdout.String(), "OTHER")
				testutil.Contains(t, stdout.String(), "Archon node wiring is not exercised")
				testutil.Contains(t, stdout.String(), "Expected marker: no failure-path test.")
				testutil.Contains(t, stdout.String(), "Result JSON: "+out)
				lines := strings.Split(strings.TrimSpace(string(testutil.Must(os.ReadFile(calls)))), "\n")
				testutil.Equal(t, len(lines), 6)
				for i, line := range lines {
					var call document
					testutil.OK(t, json.Unmarshal([]byte(line), &call))
					if i == 0 {
						testutil.Equal(t, call["kind"], "route")
					}
					if i == 5 {
						testutil.Equal(t, call["kind"], "matching")
					}
				}
				for _, name := range []string{"prepare-output.json", "changes-output.json", "route-output.json", "launch-output.json", "contract-output.json", "diff.patch", "source/base/src/change.py", "source/commit/src/change.py"} {
					_, err := os.Stat(filepath.Join(result.Cases[0].Artifacts, "judge", name))
					testutil.OK(t, err)
				}
				testutil.Equal(t, testutil.Git(t, root, "rev-list", "--count", "HEAD"), "2")
			})
		}
	}
}

func TestEvalSelectionAndTemporaryOutput(t *testing.T) {
	root := testutil.Repo(t)
	installTestPack(t, root)
	casesDir := filepath.Join(root, "cases")
	integrationCase(t, casesDir, false)
	testutil.OK(t, copyFolder(os.DirFS(filepath.Join(casesDir, "sample")), filepath.Join(casesDir, "second")))
	putCase(t, casesDir, "unselected", `{}`)
	testutil.Put(t, root, ".devkit/config.json", `{"provider":"codex","language":"fr","eval":"absent"}`)
	env, stdout, stderr, calls := fakeEnv(t, root, false)
	registry := cli.New("test", Commands())
	code := registry.Execute([]string{"eval", "judge", "--dir", "cases", "--case", "second", "--case", "sample"}, env)
	if code != 0 {
		t.Fatalf("%s\n%s", stdout, stderr)
	}
	var out string
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "Results: ") {
			out = strings.TrimPrefix(line, "Results: ")
		}
	}
	if out == "" {
		t.Fatal("temporary output path not printed")
	}
	t.Cleanup(func() { os.RemoveAll(out) })
	var result Result
	testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(out, "result.json"))), &result))
	testutil.Equal(t, []string{result.Cases[0].Name, result.Cases[1].Name}, []string{"second", "sample"})
	testutil.Equal(t, result.Totals.Expected, 6)
	testutil.Equal(t, result.Totals.Other, 2)
	lines := strings.Split(strings.TrimSpace(string(testutil.Must(os.ReadFile(calls)))), "\n")
	testutil.Equal(t, len(lines), 12)
}

func TestEvalProviderErrorsAreNotScores(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, bad := range []string{"contract", "matching"} {
			t.Run(provider+"/"+bad, func(t *testing.T) {
				root := testutil.Repo(t)
				installTestPack(t, root)
				integrationCase(t, filepath.Join(root, "cases"), false)
				testutil.Put(t, root, ".devkit/config.json", fmt.Sprintf(`{"provider":%q,"language":"fr"}`, provider))
				env, stdout, _, _ := fakeEnv(t, root, false)
				env = withEnv(env, map[string]string{"DEVKIT_EVAL_BAD": bad})
				out := t.TempDir()
				code := cli.New("test", Commands()).Execute([]string{"eval", "judge", "--dir", "cases", "--out", out}, env)
				testutil.Equal(t, code, 0)
				testutil.Contains(t, stdout.String(), "ERROR (not scored)")
				testutil.Contains(t, stdout.String(), "answer does not match output schema")
				var result Result
				testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(out, "result.json"))), &result))
				testutil.Equal(t, len(result.Errored), 1)
				testutil.Equal(t, result.Totals, Counts{})
			})
		}
	}
}

func TestEvalStandardsPrecedence(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			root := testutil.Repo(t)
			installTestPack(t, root)
			integrationCase(t, filepath.Join(root, "cases"), false)
			testutil.Put(t, root, ".devkit/config.json", `{"language":"fr","standards":"rules"}`)
			for _, name := range []string{"rules", "override"} {
				testutil.Standards(t, root, name)
				file := filepath.Join(root, name, "authority.md")
				data := testutil.Must(os.ReadFile(file))
				testutil.OK(t, os.WriteFile(file, append(data, []byte("\nStandards marker: "+name+"\n")...), 0644))
			}
			env, stdout, stderr, _ := fakeEnv(t, root, false)
			marker := "rules"
			args := []string{"eval", "judge", "--dir", "cases", "--out", t.TempDir()}
			if explicit {
				marker = "override"
				args = append(args, "--standards", "override")
			}
			env = withEnv(env, map[string]string{"DEVKIT_EVAL_STANDARDS": marker})
			if code := cli.New("test", Commands()).Execute(args, env); code != 0 {
				t.Fatalf("%s\n%s", stdout, stderr)
			}
		})
	}
}

func TestMatchingValidationAndEmptyCases(t *testing.T) {
	c := Case{Name: "clean", Expected: []Finding{}, NotExpected: []Finding{}}
	answer := document{"expected": document{}, "not_expected": document{}}
	result := testutil.Must(aggregate(c, []string{"scope"}, []Reported{}, answer))
	testutil.Equal(t, result.Totals, Counts{})
	c.Expected = []Finding{{ID: "a", Quality: "data", File: "src/a.py", Summary: "Bad state"}, {ID: "b", Quality: "data", File: "src/a.py", Summary: "Same failure"}}
	reported := []Reported{{ID: "r1", Quality: "data", File: "src/a.py"}, {ID: "r2", Quality: "data", File: "src/a.py"}}
	result = testutil.Must(aggregate(c, []string{"data"}, reported, document{"expected": document{"a": "r1", "b": "r1"}, "not_expected": document{}}))
	testutil.Equal(t, result.Totals, Counts{Expected: 2, Found: 2, Other: 1})
	for _, invalid := range []document{
		{"expected": document{"a": "r1"}, "not_expected": document{}},
		{"expected": document{"a": "r1", "b": "unknown"}, "not_expected": document{}},
		{"expected": document{"a": "r1", "b": ""}, "not_expected": document{}, "extra": true},
	} {
		_, err := aggregate(c, []string{"data"}, reported, invalid)
		testutil.Fails(t, err, "matching")
	}
	reported[0].Quality = "scope"
	_, err := aggregate(c, []string{"scope"}, reported, document{"expected": document{"a": "r1", "b": ""}, "not_expected": document{}})
	testutil.Fails(t, err, "matching")
	reported[0].Quality, reported[0].File = "data", "src/related.py"
	result = testutil.Must(aggregate(c, []string{"data"}, reported, document{"expected": document{"a": "r1", "b": ""}, "not_expected": document{}}))
	testutil.Equal(t, result.Totals.Found, 1)
}

func TestEvalCLIHelper(t *testing.T) {
	if os.Getenv("DEVKIT_EVAL_FAKE") != "1" {
		return
	}
	args := os.Args[slices.Index(os.Args, "--")+1:]
	prompt := string(testutil.Must(io.ReadAll(os.Stdin)))
	for _, variable := range []string{"$PACK_DIR", "$ARTIFACTS_DIR", "$INPUTS.quality"} {
		if strings.Contains(prompt, variable) {
			t.Fatalf("unresolved %s", variable)
		}
	}
	provider := "claude"
	var schema document
	var answerPath string
	if args[0] == "exec" {
		provider = "codex"
		testutil.Equal(t, len(args), 11)
		testutil.Equal(t, args[:4], []string{"exec", "--sandbox", "read-only", "--cd"})
		testutil.Equal(t, testutil.Must(filepath.EvalSymlinks(args[4])), testutil.Must(os.Getwd()))
		testutil.Equal(t, args[5], "--output-schema")
		testutil.Equal(t, args[7], "-o")
		testutil.Equal(t, args[9:], []string{"--skip-git-repo-check", "-"})
		testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(args[6])), &schema))
		answerPath = args[8]
		_, err := os.Stat(".git")
		testutil.Equal(t, os.IsNotExist(err), true)
	} else {
		testutil.Equal(t, len(args), 13)
		testutil.Equal(t, args[:2], []string{"-p", "--json-schema"})
		testutil.Equal(t, args[3:11], []string{"--allowedTools", "Read,Grep,Glob", "--tools", "Read,Grep,Glob", "--permission-mode", "dontAsk", "--output-format", "json"})
		testutil.Equal(t, args[11], "--add-dir")
		testutil.OK(t, json.Unmarshal([]byte(args[2]), &schema))
	}
	artifacts := os.Getenv("ARTIFACTS_DIR")
	readDoc := func(name string) document {
		var d document
		testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(filepath.Join(artifacts, name))), &d))
		return d
	}
	kind, answer := "", document{}
	if strings.HasPrefix(prompt, "Match evaluation findings.") {
		kind = "matching"
		input := readDoc("matching/input.json")
		testutil.Equal(t, input["language"], "fr")
		testutil.Equal(t, len(input["reported"].([]any)), 3)
		testutil.Equal(t, len(input["expected"].([]any)), 3)
		found, falsePositive := "", ""
		for _, raw := range input["reported"].([]any) {
			f := raw.(map[string]any)
			for _, key := range []string{"quality", "file", "line", "failure_scenario", "evidence"} {
				if _, ok := f[key]; !ok {
					t.Fatalf("missing matching material %s", key)
				}
			}
			if f["quality"] == "contract" {
				found = f["id"].(string)
			}
			if f["quality"] == "scope" {
				falsePositive = f["id"].(string)
			}
		}
		answer = document{"expected": document{"found": found, "routing": "", "missed": ""}, "not_expected": document{"false-positive": falsePositive}}
	} else {
		if strings.Contains(prompt, "Expected marker") {
			t.Fatal("expectation leaked into judge")
		}
		_, err := os.Stat(filepath.Join(artifacts, "matching/input.json"))
		testutil.Equal(t, os.IsNotExist(err), true)
		target := readDoc("judge/target.json")
		tree := target["tree"].(string)
		if provider == "codex" {
			testutil.Equal(t, testutil.Must(os.Getwd()), testutil.Must(filepath.EvalSymlinks(tree)))
		} else {
			testutil.Equal(t, args[12], tree)
		}
		testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(tree, "src/unchanged.py")))), "def peer(): return 3\n")
		testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "judge/source/base/src/change.py")))), "def value(): return 1\n")
		testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "judge/source/commit/src/change.py")))), "def value(): return 2\n")
		if strings.Contains(prompt, "When Jev is unavailable") {
			kind = "route"
			testutil.Contains(t, prompt, filepath.Join(artifacts, "judge/diff.patch"))
			answer = document{"qualities": []document{{"quality": "contract", "file": "src/change.py", "reason": "Changed response"}, {"quality": "test", "file": "src/change.py", "reason": "Source code"}, {"quality": "readability", "file": "src/change.py", "reason": "Source code"}}}
		} else {
			kind = os.Getenv("QUALITY")
			testutil.Contains(t, prompt, "Installed quality prompt marker.")
			testutil.Contains(t, prompt, filepath.Join(artifacts, "judge/materials/"+kind+".json"))
			if schema["properties"].(map[string]any)["eval_probe"] == nil {
				t.Fatal("schema was not read from installed YAML")
			}
			material := readDoc("judge/materials/" + kind + ".json")
			testutil.Equal(t, material["language"], "fr")
			testutil.Equal(t, material["quality"], kind)
			testutil.Equal(t, material["tree"], tree)
			testutil.Contains(t, fmt.Sprint(material["criteria"]), "review/"+kind+".md")
			if os.Getenv("DEVKIT_EVAL_CHECKS") == "true" {
				testutil.Contains(t, fmt.Sprint(material["checks"]), "exit 99")
			}
			if marker := os.Getenv("DEVKIT_EVAL_STANDARDS"); marker != "" {
				file := filepath.Join(artifacts, "criteria", target["standards"].(string), "authority.md")
				testutil.Contains(t, string(testutil.Must(os.ReadFile(file))), "Standards marker: "+marker)
			}
			_, err := os.Stat(filepath.Join(artifacts, "judge/plan.json"))
			testutil.Equal(t, os.IsNotExist(err), true)
			_, err = os.Stat(filepath.Join(artifacts, "judge/requirements.json"))
			testutil.Equal(t, err == nil, os.Getenv("DEVKIT_EVAL_REQUIREMENTS") == "true")
			if err == nil {
				testutil.Equal(t, readDoc("judge/requirements.json")["goal"], "Preserve all other behavior while changing the returned value.")
			}
			findings := []document{}
			if kind != "test" {
				findings = append(findings, document{"aspect": kind, "file": "src/change.py", "line": "1", "blocking": true, "needs_decision": false, "deferred": false, "mechanical": false, "failure_scenario": "Observed failure", "evidence": "return 2", "fix": "Preserve the contract"})
			}
			answer = document{"criteria": []document{{"aspect": kind, "verdict": "fail", "checked": "Read code", "not_checked": ""}}, "findings": findings, "eval_probe": "installed"}
		}
	}
	if kind == os.Getenv("DEVKIT_EVAL_BAD") {
		answer = document{"invalid": true}
	} else {
		testutil.OK(t, validate(schema, answer))
	}
	file := testutil.Must(os.OpenFile(os.Getenv("DEVKIT_EVAL_CALLS"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600))
	testutil.OK(t, json.NewEncoder(file).Encode(document{"kind": kind, "provider": provider}))
	testutil.OK(t, file.Close())
	if provider == "codex" {
		testutil.OK(t, os.WriteFile(answerPath, testutil.Must(json.Marshal(answer)), 0600))
	} else {
		testutil.OK(t, json.NewEncoder(os.Stdout).Encode(document{"type": "result", "subtype": "success", "is_error": false, "structured_output": answer}))
	}
	os.Exit(0)
}
