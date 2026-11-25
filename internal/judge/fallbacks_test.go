package judge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestJudgeRejectsLostOrMismatchedExpectedPlan(t *testing.T) {
	for _, command := range []string{"completion", "changes", "launch", "finish"} {
		for _, input := range []string{"missing", `null`, `{"version":2,"tests":[]}`} {
			t.Run(command+"/"+input, func(t *testing.T) {
				repo, dir, vars := finishFixture(t)
				vars["PREPARE"] = jsonText(runCLI(t, repo, vars, "prepare"))
				vars["CHANGES"] = jsonText(runCLI(t, repo, vars, "changes"))
				vars["ROUTE"] = `{"qualities":[]}`
				collectPassingJudges(t, repo, vars)
				vars["COMPLETION"] = jsonText(runCLI(t, repo, vars, "completion"))
				vars["GATE"] = `{"passed":true,"failing":[],"results":"checks/1.jsonl"}`
				judgeDir := filepath.Join(vars["ARTIFACTS_DIR"], "judge")
				target := must(read(filepath.Join(judgeDir, "target.json")))
				target["plan_version"] = 1
				testutil.OK(t, save(filepath.Join(judgeDir, "target.json"), target))
				if input != "missing" {
					testutil.Put(t, judgeDir, "plan.json", input)
				}
				before := artifactSnapshot(t, dir, vars["ARTIFACTS_DIR"])
				var stdout, stderr bytes.Buffer
				env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
				err := cli.New("test", Commands()).Run([]string{"judge", command}, env)
				testutil.Fails(t, err, filepath.Join(judgeDir, "plan.json"))
				testutil.Equal(t, artifactSnapshot(t, dir, vars["ARTIFACTS_DIR"]), before)
			})
		}
	}
}

func TestCompletionDistinguishesMissingPathFromBrokenBlob(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "test.ext", "planned test\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "test source")
	g := must(gitx.New(repo, "main"))
	plan := doc{"tests": []doc{{"method": "test", "file": "absent.ext", "name": "planned test"}}}
	testutil.Equal(t, must(completion(plan, "HEAD", g))["missing_count"], 1)
	docs(plan["tests"])[0]["file"] = "test.ext"
	blob := testutil.Git(t, repo, "rev-parse", "HEAD:test.ext")
	testutil.OK(t, os.Remove(filepath.Join(repo, ".git/objects", blob[:2], blob[2:])))
	_, err := completion(plan, "HEAD", g)
	testutil.Fails(t, err, "test.ext")
}

func TestFinishRequiresNamedChecksResultsAndAllowsEmptyResults(t *testing.T) {
	for _, state := range []string{"missing", "empty"} {
		t.Run(state, func(t *testing.T) {
			repo, dir, vars := finishFixture(t)
			vars["PREPARE"] = jsonText(runCLI(t, repo, vars, "prepare"))
			vars["CHANGES"] = jsonText(runCLI(t, repo, vars, "changes"))
			vars["ROUTE"] = `{"qualities":[]}`
			collectPassingJudges(t, repo, vars)
			vars["COMPLETION"] = jsonText(runCLI(t, repo, vars, "completion"))
			path := filepath.Join(vars["ARTIFACTS_DIR"], "checks/1.jsonl")
			gate := doc{"passed": true, "failing": []string{}, "results": path}
			vars["GATE"] = jsonText(gate)
			if state == "empty" {
				testutil.Put(t, vars["ARTIFACTS_DIR"], "checks/1.jsonl", "")
			}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			err := cli.New("test", Commands()).Run([]string{"judge", "finish"}, env)
			if state == "missing" {
				testutil.Fails(t, err, path)
				testutil.Equal(t, len(must(record.List(dir, "result"))), 0)
				testutil.Equal(t, len(must(record.List(dir, "judgment"))), 0)
			} else {
				testutil.OK(t, err)
				testutil.Equal(t, len(must(record.List(dir, "result"))), 1)
			}
		})
	}
}

func TestRouteReportsJevArtifactWriteFailureEvenWhenUnavailable(t *testing.T) {
	repo := testutil.Repo(t)
	a := jevInput(t)
	path := filepath.Join(a, "judge/route-jev.json")
	testutil.OK(t, os.Mkdir(path, 0755))
	vars := map[string]string{"ARTIFACTS_DIR": a, "DEVKIT_CLAUDE": "/usr/bin/false"}
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	err := runRouteTimeout(env, time.Second)
	testutil.Fails(t, err, path)
	testutil.Equal(t, out.Len(), 0)
}

func TestCompletionAcceptsMatchingPlanWithNoTests(t *testing.T) {
	repo, _, vars := finishFixture(t)
	runCLI(t, repo, vars, "prepare")
	dir := filepath.Join(vars["ARTIFACTS_DIR"], "judge")
	target := must(read(filepath.Join(dir, "target.json")))
	target["plan_version"] = 1
	testutil.OK(t, save(filepath.Join(dir, "target.json"), target))
	testutil.OK(t, save(filepath.Join(dir, "plan.json"), doc{"version": 1, "tests": []doc{}}))
	testutil.Equal(t, runCLI(t, repo, vars, "completion")["checked_count"], 0)
}

func TestChangesRejectsSourceReadFailureInsteadOfOmittingSide(t *testing.T) {
	repo, _, vars := finishFixture(t)
	runCLI(t, repo, vars, "prepare")
	realGit := must(exec.LookPath("git"))
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \"$*\" in\n  'cat-file -e '*:sample.ext|'show '*:sample.ext) printf 'source read failure\\n' >&2; exit 128;;\nesac\nexec '" + realGit + "' \"$@\"\n"
	testutil.Put(t, bin, "git", script)
	testutil.OK(t, os.Chmod(filepath.Join(bin, "git"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := writeChanges(repo, vars["ARTIFACTS_DIR"])
	testutil.Fails(t, err, "sample.ext")
	testutil.Fails(t, err, "source read failure")
}
