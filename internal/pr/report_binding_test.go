package pr

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestReportJudgmentBindsPublishedTitleAndBody(t *testing.T) {
	for _, kind := range []string{"create", "edit", "rejudged", "snapshot", "assemble-body", "assemble-title", "assemble-identical", "body-bytes", "title-bytes", "missing-body", "missing-title", "missing-state", "legacy-judgment", "failed-rejudgment", "assemble-write-failure"} {
		t.Run(kind, func(t *testing.T) {
			repo := testutil.Repo(t)
			for key, value := range map[string]string{"user.name": "Test", "user.email": "test@example.invalid", "commit.gpgsign": "false", "core.hooksPath": "/dev/null"} {
				testutil.Git(t, repo, "config", key, value)
			}
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			base := testutil.Git(t, repo, "rev-parse", "main")
			remote := t.TempDir()
			testutil.Git(t, remote, "init", "--bare", "-q")
			testutil.Git(t, repo, "remote", "add", "origin", remote)
			testutil.Git(t, repo, "push", "-q", "origin", "HEAD:refs/heads/feature/x")
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			must(record.Append(dir, "request", doc{"run_id": "r1", "recorded_at": "t", "owner_words": "Request", "supplement": "", "criteria_commit": base, "issue": 42}))
			addRun(dir, "pr1", "pr")
			testutil.Put(t, artifacts, "pr/facts.json", jsonText(doc{"head": head, "base": base, "versions": versions()}))
			testutil.Put(t, artifacts, "pr/lists.json", `{}`)
			vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "pr1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main", "INPUTS_TEXT": jsonText(writerText()), "INPUTS_JUDGMENT": `{"verdict":"pass","findings":[]}`}
			runCLI(t, repo, vars, "assemble")
			testutil.Equal(t, runCLI(t, repo, vars, "judgment")["passed"], true)
			titlePath := filepath.Join(artifacts, "pr/pr-title.txt")
			bodyPath := filepath.Join(artifacts, "pr/pr.md")
			statePath := filepath.Join(artifacts, "pr/report-state.json")
			title, body := must(os.ReadFile(titlePath)), must(os.ReadFile(bodyPath))
			testutil.Equal(t, strings.HasSuffix(string(body), "\n\nCloses #42\n"), true)
			judged := must(record.Latest(dir, "judgment"))
			bound := stringsOf(judged["materials"])
			testutil.Equal(t, bound[len(bound)-2:], []string{fmt.Sprintf("pr/pr-title.txt#sha256=%x", sha256.Sum256(title)), fmt.Sprintf("pr/pr.md#sha256=%x", sha256.Sum256(body))})
			if object(judged["target"])["change_id"] != nil {
				t.Fatal("report content was stored as code change identity")
			}
			success := kind == "create" || kind == "edit" || kind == "rejudged" || kind == "snapshot"
			switch kind {
			case "assemble-body", "assemble-title", "assemble-identical", "rejudged", "assemble-write-failure":
				writer := writerText()
				if kind == "assemble-title" {
					writer["title"] = "Unjudged title"
				} else if kind != "assemble-identical" {
					writer["opening"] = "Different report"
				}
				if kind == "rejudged" {
					writer["title"] = "Rejudged title"
				}
				vars["INPUTS_TEXT"] = jsonText(writer)
				if kind == "assemble-write-failure" {
					testutil.OK(t, os.Remove(titlePath))
					testutil.OK(t, os.Mkdir(titlePath, 0755))
					var stdout, stderr bytes.Buffer
					env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
					if cli.New("test", Commands()).Execute([]string{"pr", "assemble"}, env) == 0 {
						t.Fatal("assemble accepted an unwritable title")
					}
				} else {
					runCLI(t, repo, vars, "assemble")
				}
				testutil.Equal(t, must(read(statePath))["passed"], false)
				testutil.Equal(t, len(must(record.List(dir, "judgment"))), 1)
				if kind == "rejudged" {
					round := runCLI(t, repo, vars, "judgment")
					testutil.Equal(t, round["passed"], true)
					testutil.Equal(t, round["round"], 2)
					title, body = must(os.ReadFile(titlePath)), must(os.ReadFile(bodyPath))
				}
			case "body-bytes":
				testutil.OK(t, os.WriteFile(bodyPath, append(body, '\n'), 0644))
			case "title-bytes":
				testutil.OK(t, os.WriteFile(titlePath, append(title, ' '), 0644))
			case "missing-body":
				testutil.OK(t, os.Remove(bodyPath))
			case "missing-title":
				testutil.OK(t, os.Remove(titlePath))
			case "missing-state":
				testutil.OK(t, os.Remove(statePath))
			case "legacy-judgment":
				judged["materials"] = materials
				must(record.Append(dir, "judgment", judged))
			case "failed-rejudgment":
				vars["INPUTS_JUDGMENT"] = `{"verdict":"fail","findings":[]}`
				testutil.Equal(t, runCLI(t, repo, vars, "judgment")["passed"], false)
			}
			before := testutil.Git(t, repo, "status", "--porcelain", "--untracked-files=all")
			index := testutil.Git(t, repo, "write-tree")
			judgments := must(record.List(dir, "judgment"))
			reportState, err := os.ReadFile(statePath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			realGit := must(exec.LookPath("git"))
			bin := t.TempDir()
			executable := must(os.Executable())
			for _, name := range []string{"git", "gh"} {
				testutil.OK(t, os.Symlink(executable, filepath.Join(bin, name)))
			}
			calls := filepath.Join(t.TempDir(), "calls.jsonl")
			published := filepath.Join(t.TempDir(), "body")
			t.Setenv("DEVKIT_PR_TEST_TOOL", "1")
			t.Setenv("DEVKIT_PR_TEST_CALLS", calls)
			t.Setenv("DEVKIT_PR_TEST_GIT", realGit)
			t.Setenv("DEVKIT_PR_TEST_BODY", published)
			if success {
				t.Setenv("DEVKIT_PR_TEST_PUBLISH", "1")
			}
			if kind == "edit" {
				t.Setenv("DEVKIT_PR_TEST_URL", "https://example.invalid/pr/1")
			}
			if kind == "snapshot" {
				t.Setenv("DEVKIT_PR_TEST_MUTATE_BODY", bodyPath)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			var stdout, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			code := cli.New("test", Commands()).Execute([]string{"pr", "open"}, env)
			testutil.Equal(t, code == 0, success)
			var publication []string
			pushed := false
			for _, line := range strings.Split(strings.TrimSpace(string(must(os.ReadFile(calls)))), "\n") {
				var args []string
				testutil.OK(t, json.Unmarshal([]byte(line), &args))
				if !success && (args[0] == "gh" || len(args) > 1 && (args[1] == "commit" || args[1] == "push" || args[1] == "ls-remote")) {
					t.Fatalf("publication called %v after rejection", args)
				}
				if args[0] == "git" && len(args) > 1 && args[1] == "push" {
					pushed = true
				}
				if args[0] == "gh" && len(args) > 2 && args[2] != "list" {
					publication = args
				}
			}
			if success {
				testutil.Equal(t, pushed, true)
				want := "create"
				if kind == "edit" {
					want = "edit"
				}
				testutil.Equal(t, publication[2], want)
				testutil.Contains(t, strings.Join(publication, "\n"), "--title\n"+strings.TrimSpace(string(title))+"\n--body-file\n-")
				testutil.Equal(t, string(must(os.ReadFile(published))), string(body))
				testutil.Equal(t, testutil.Git(t, remote, "rev-parse", "refs/heads/feature/x"), testutil.Git(t, repo, "rev-parse", "HEAD"))
				testutil.Equal(t, must(read(filepath.Join(artifacts, "pr/outcome.json")))["outcome"], "pass")
			} else {
				if kind == "body-bytes" || kind == "title-bytes" || kind == "legacy-judgment" {
					testutil.Contains(t, stderr.String(), "differs from the report judgment")
				}
				testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
				testutil.Equal(t, testutil.Git(t, remote, "rev-parse", "refs/heads/feature/x"), head)
				testutil.Equal(t, testutil.Git(t, repo, "write-tree"), index)
				testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain", "--untracked-files=all"), before)
				testutil.Equal(t, must(record.List(dir, "judgment")), judgments)
				currentState, err := os.ReadFile(statePath)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				testutil.Equal(t, string(currentState), string(reportState))
				_, err = os.Stat(filepath.Join(repo, ".devkit/backlog/feature%2Fx.json"))
				testutil.Equal(t, os.IsNotExist(err), true)
				_, err = os.Stat(filepath.Join(artifacts, "pr/outcome.json"))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
		})
	}
}

func TestReportJudgmentRequiresBothMaterialFiles(t *testing.T) {
	for _, missing := range []string{"pr-title.txt", "pr.md"} {
		t.Run(missing, func(t *testing.T) {
			repo := testutil.Repo(t)
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			base := testutil.Git(t, repo, "rev-parse", "main")
			must(record.Append(dir, "request", doc{"run_id": "r1", "recorded_at": "t", "owner_words": "Request", "supplement": "", "criteria_commit": base}))
			addRun(dir, "pr1", "pr")
			testutil.Put(t, artifacts, "pr/facts.json", jsonText(doc{"head": testutil.Git(t, repo, "rev-parse", "HEAD"), "base": base, "versions": versions()}))
			testutil.Put(t, artifacts, "pr/lists.json", `{}`)
			vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "pr1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main", "INPUTS_TEXT": jsonText(writerText()), "INPUTS_JUDGMENT": `{"verdict":"pass","findings":[]}`}
			runCLI(t, repo, vars, "assemble")
			testutil.OK(t, os.Remove(filepath.Join(artifacts, "pr", missing)))
			var stdout, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			if cli.New("test", Commands()).Execute([]string{"pr", "judgment"}, env) == 0 {
				t.Fatal("judgment accepted a missing material")
			}
			testutil.Contains(t, stderr.String(), missing)
			testutil.Equal(t, len(must(record.List(dir, "judgment"))), 0)
			for _, file := range []string{"report-state.json", "report-findings.json"} {
				_, err := os.Stat(filepath.Join(artifacts, "pr", file))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
		})
	}
}
