package judge

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func finishFixture(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, "sample.ext", "before\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "standards and source")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	base := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Put(t, repo, "sample.ext", "after\n")
	testutil.Git(t, repo, "commit", "-qam", "change")
	state, artifacts := t.TempDir(), t.TempDir()
	dir := record.RecordDir(state, "feature/x")
	started := must(record.Start(dir, "judge", record.StartOptions{RunID: "judge-1", Commit: testutil.Git(t, repo, "rev-parse", "HEAD"), BaseCommit: base}))
	testutil.Equal(t, started.Stopped, false)
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "judge-1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
	testutil.Put(t, repo, "checks/1.jsonl", "")
	testutil.Put(t, repo, ".git/info/exclude", "checks/\n")
	return repo, dir, vars
}

func collectPassingJudges(t *testing.T, repo string, vars map[string]string) {
	t.Helper()
	launch := runCLI(t, repo, vars, "launch")
	changes := vars["CHANGES"]
	vars["CHANGES"] = jsonText(launch)
	for _, q := range qualities() {
		if !truth(launch["launch_"+q]) {
			continue
		}
		material := must(read(filepath.Join(vars["ARTIFACTS_DIR"], "judge/materials", q+".json")))
		criteria := []doc{}
		for _, path := range stringsOf(material["criteria"]) {
			criteria = append(criteria, doc{"aspect": strings.TrimSuffix(filepath.Base(path), ".md"), "verdict": "pass", "checked": "source", "not_checked": ""})
		}
		vars["JUDGE_"+strings.ToUpper(q)] = jsonText(doc{"criteria": criteria, "findings": []doc{}})
	}
	vars["COLLECT"] = jsonText(runCLI(t, repo, vars, "collect"))
	vars["CHANGES"] = changes
}

func artifactSnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, root := range roots {
		testutil.OK(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				files[path] = string(must(os.ReadFile(path)))
			}
			return nil
		}))
	}
	return files
}

func TestFinishRejectsLostArtifactsWithoutWriting(t *testing.T) {
	for _, tc := range []struct{ name, file, replacement, want string }{
		{"changes", "changes.json", "", "changes.json"},
		{"completion", "completion.json", "", "completion.json"},
		{"findings", "findings.json", "", "findings.json"},
		{"launch", "launched.json", "", "launched.json"},
		{"null changes", "changes.json", "null", "changes.json"},
		{"null completion", "completion.json", "null", "planned test results"},
		{"missing test observations", "completion.json", "{}", "planned test results"},
		{"null findings", "findings.json", "null", "did not finish"},
		{"missing judge result", "findings.json", `{"judges":[]}`, "did not finish"},
		{"missing launched test result", "findings.json", `{"judges":[{"judge":"scope","criteria":[{"aspect":"scope","verdict":"pass","checked":"source","not_checked":""}],"findings":[]}]}`, "test did not finish"},
		{"null launch", "launched.json", "null", "launched judges"},
		{"empty launch", "launched.json", `{"launched":[]}`, "launched judges"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, dir, vars := finishFixture(t)
			vars["PREPARE"] = jsonText(runCLI(t, repo, vars, "prepare"))
			vars["CHANGES"] = jsonText(runCLI(t, repo, vars, "changes"))
			vars["ROUTE"] = `{"qualities":[{"quality":"test","file":"sample.ext","reason":"source"}]}`
			collectPassingJudges(t, repo, vars)
			findings := must(read(filepath.Join(vars["ARTIFACTS_DIR"], "judge/findings.json")))
			docs(findings["judges"])[0]["findings"] = []doc{finding(nil)}
			testutil.OK(t, save(filepath.Join(vars["ARTIFACTS_DIR"], "judge/findings.json"), findings))
			testutil.OK(t, save(filepath.Join(vars["ARTIFACTS_DIR"], "judge/plan.json"), doc{"tests": []doc{{"criterion": "C1", "method": "test", "file": "missing_test.ext", "name": "planned test"}}}))
			vars["COMPLETION"] = jsonText(runCLI(t, repo, vars, "completion"))
			vars["GATE"] = `{"passed":true,"failing":[],"results":"checks/1.jsonl"}`
			path := filepath.Join(vars["ARTIFACTS_DIR"], "judge", tc.file)
			if tc.replacement == "" {
				testutil.OK(t, os.Remove(path))
			} else {
				testutil.OK(t, os.WriteFile(path, []byte(tc.replacement), 0644))
			}
			before := artifactSnapshot(t, dir, vars["ARTIFACTS_DIR"])
			var stdout, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { value, ok := vars[key]; return value, ok }}
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"judge", "finish"}, env), 1)
			testutil.Contains(t, stderr.String(), tc.want)
			testutil.Equal(t, stdout.Len(), 0)
			testutil.Equal(t, artifactSnapshot(t, dir, vars["ARTIFACTS_DIR"]), before)
			testutil.Equal(t, len(must(record.List(dir, "result"))), 0)
			testutil.Equal(t, len(must(record.List(dir, "judgment"))), 0)
		})
	}
}

func TestFinishPreservesEarlyStopsWithoutLaterArtifacts(t *testing.T) {
	for _, stage := range []string{"prepare", "changes"} {
		t.Run(stage, func(t *testing.T) {
			repo, dir, vars := finishFixture(t)
			if stage == "prepare" {
				testutil.Put(t, repo, "sample.ext", "dirty\n")
			} else {
				testutil.Put(t, repo, ".devkit/config.json", `{"decision_paths":[{"paths":["sample.ext"],"quality":"contract","ask":true}]}`)
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "decision rule")
				must(record.Append(dir, "requirements", doc{"request": 1, "run_id": "requirements-1", "recorded_at": "t", "goal": "Review the change", "completion_conditions": []doc{{"id": "C1", "text": "Preserve the contract"}}, "constraints": []string{}, "open_points": []doc{}}))
				must(record.Append(dir, "plan", doc{"requirements_version": 1, "run_id": "plan-1", "recorded_at": "t", "skeleton_commit": testutil.Git(t, repo, "rev-parse", "main"), "decisions": []doc{}, "increments": []doc{{"id": "U1", "summary": "Preserve the contract", "criteria": []string{"C1"}}}, "tests": []doc{{"criterion": "C1", "method": "owner_check"}}, "kept_behaviors": []doc{}}))
			}
			vars["PREPARE"] = jsonText(runCLI(t, repo, vars, "prepare"))
			if stage == "changes" {
				changes := runCLI(t, repo, vars, "changes")
				testutil.Equal(t, changes["proceed"], false)
				vars["CHANGES"] = jsonText(changes)
			}
			result := runCLI(t, repo, vars, "finish")
			if stage == "prepare" {
				testutil.Equal(t, result["outcome"], "stop")
				testutil.Contains(t, text(object(result["stop"])["state"]), "working tree")
			} else {
				testutil.Equal(t, result["outcome"], "back")
				testutil.Equal(t, object(result["back"])["to"], "plan")
			}
			testutil.Equal(t, len(must(record.List(dir, "result"))), 1)
			testutil.Equal(t, len(must(record.List(dir, "judgment"))), 0)
			for _, name := range []string{"completion.json", "findings.json", "launched.json"} {
				_, err := os.Stat(filepath.Join(vars["ARTIFACTS_DIR"], "judge", name))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
		})
	}
}

func TestFinishPreservesCompleteArtifactOutcomes(t *testing.T) {
	for _, reason := range []string{"pass", "blocking finding", "missing planned test"} {
		t.Run(reason, func(t *testing.T) {
			repo, dir, vars := finishFixture(t)
			vars["PREPARE"] = jsonText(runCLI(t, repo, vars, "prepare"))
			vars["CHANGES"] = jsonText(runCLI(t, repo, vars, "changes"))
			vars["ROUTE"] = `{"qualities":[]}`
			collectPassingJudges(t, repo, vars)
			if reason == "blocking finding" {
				findings := must(read(filepath.Join(vars["ARTIFACTS_DIR"], "judge/findings.json")))
				docs(findings["judges"])[0]["findings"] = []doc{finding(nil)}
				testutil.OK(t, save(filepath.Join(vars["ARTIFACTS_DIR"], "judge/findings.json"), findings))
			}
			if reason == "missing planned test" {
				testutil.OK(t, save(filepath.Join(vars["ARTIFACTS_DIR"], "judge/plan.json"), doc{"tests": []doc{{"criterion": "C1", "method": "test", "file": "missing_test.ext", "name": "planned test"}}}))
			}
			vars["COMPLETION"] = jsonText(runCLI(t, repo, vars, "completion"))
			vars["GATE"] = `{"passed":true,"failing":[],"results":"checks/1.jsonl"}`
			result := runCLI(t, repo, vars, "finish")
			want := "stop"
			if reason == "pass" {
				want = "pass"
			}
			testutil.Equal(t, result["outcome"], want)
			testutil.Equal(t, len(must(record.List(dir, "result"))), 1)
			testutil.Equal(t, len(must(record.List(dir, "judgment"))), 2)
		})
	}
}
