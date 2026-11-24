package judge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestJudgeCLIUsesFixedSourcePathsAfterRelocation(t *testing.T) {
	for _, standards := range []string{".devkit/standards", "original-policy"} {
		for _, reuse := range []bool{false, true} {
			name := standards + "/fresh"
			if reuse {
				name = standards + "/reused"
			}
			t.Run(name, func(t *testing.T) {
				repo := testutil.Repo(t)
				testutil.Put(t, repo, ".devkit/config.json", jsonText(doc{"standards": standards, "language": "fr"}))
				testutil.Standards(t, repo, standards)
				testutil.Put(t, repo, filepath.Join(standards, "authority.md"), "fixed authority\n")
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "fixed criteria")
				testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
				base := testutil.Git(t, repo, "rev-parse", "HEAD")
				state := t.TempDir()
				dir := record.RecordDir(state, "feature/x")
				must(record.Start(dir, "judge", record.StartOptions{RunID: "judge-1", Commit: base, BaseCommit: base}))
				vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "judge-1", "ARTIFACTS_DIR": t.TempDir(), "BASE_BRANCH": "main", "PACK_DIR": must(filepath.Abs("../../kit/.archon/workflows/dev")), "DEVKIT_CODEX": filepath.Join(t.TempDir(), "missing"), "DEVKIT_CLAUDE": filepath.Join(t.TempDir(), "missing")}
				testutil.Equal(t, runCLI(t, repo, vars, "prepare")["stopped"], false)
				fixed := must(os.ReadFile(filepath.Join(vars["ARTIFACTS_DIR"], "criteria/source.json")))
				testutil.Git(t, repo, "mv", standards, "policy")
				testutil.Put(t, repo, "policy/authority.md", "current authority\n")
				testutil.Put(t, repo, "policy/concerns.json", `{}`)
				testutil.Put(t, repo, ".devkit/config.json", `{"standards":"policy","language":"ja","checks":[{"name":"current","command":"true","timeout_seconds":10}]}`)
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "relocate criteria")
				head := testutil.Git(t, repo, "rev-parse", "HEAD")
				if !reuse {
					vars["ARTIFACTS_DIR"] = t.TempDir()
				}
				artifacts := vars["ARTIFACTS_DIR"]
				vars["ARTIFACTS_DIR"] = must(filepath.Rel(repo, artifacts))
				prepared := runCLI(t, repo, vars, "prepare")
				vars["PREPARE"] = jsonText(prepared)
				testutil.Equal(t, prepared["stopped"], false)
				target := must(read(filepath.Join(artifacts, "judge/target.json")))
				testutil.Equal(t, target["standards"], standards)
				testutil.Equal(t, target["criteria_commit"], base)
				testutil.Equal(t, target["language"], "ja")
				testutil.Equal(t, must(os.ReadFile(filepath.Join(artifacts, "criteria/source.json"))), fixed)
				testutil.Equal(t, string(must(os.ReadFile(filepath.Join(artifacts, "criteria", standards, "authority.md")))), "fixed authority\n")
				vars["CHANGES"] = jsonText(runCLI(t, repo, vars, "changes"))
				vars["ROUTE"] = jsonText(runCLI(t, repo, vars, "route"))
				testutil.Equal(t, must(read(filepath.Join(artifacts, "judge/changes.json")))["standards"], standards)
				mapping := must(concerns.Load(filepath.Join(artifacts, "criteria", standards)))
				folder := filepath.Join(vars["ARTIFACTS_DIR"], "criteria", standards)
				for _, q := range mapping.Judged("code") {
					material := must(read(filepath.Join(artifacts, "judge/materials", q+".json")))
					testutil.Equal(t, material["standards"], standards)
					testutil.Equal(t, material["criteria_folder"], folder)
					testutil.Equal(t, material["language"], "ja")
					testutil.Equal(t, docs(material["checks"])[0]["name"], "current")
					selected := mapping.Materials(folder, q, "code")
					testutil.Equal(t, material["criteria"], selected.Criteria)
					testutil.Equal(t, material["principles"], selected.Principles)
					testutil.Equal(t, material["every_quality"], selected.EveryQuality)
					testutil.Equal(t, material["common"], selected.Common)
					testutil.Equal(t, material["authority"], selected.Authority)
				}
				vars["ARTIFACTS_DIR"] = artifacts
				vars["GATE"] = `{"passed":true,"failing":[],"results":"checks/1.jsonl"}`
				vars["COMPLETION"] = jsonText(runCLI(t, repo, vars, "completion"))
				collectPassingJudges(t, repo, vars)
				testutil.Equal(t, runCLI(t, repo, vars, "finish")["outcome"], "pass")
				for _, judgment := range must(record.List(dir, "judgment")) {
					testutil.Equal(t, judgment["criteria_commit"], base)
					testutil.Equal(t, object(judgment["target"])["commit"], head)
				}
				testutil.Equal(t, must(record.List(dir, "request"))[0]["criteria_commit"], base)
				testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
				testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
			})
		}
	}
}

func TestJudgePrepareCLIRejectsInvalidFixedSourceWithoutPublishing(t *testing.T) {
	for _, tc := range []struct{ name, config, cached, want string }{
		{"invalid fixed config", `{"standards":123}`, "", "invalid .devkit/config.json"},
		{"missing fixed standards", `{"standards":"missing"}`, "", "missing does not exist at"},
		{"missing source paths", `{}`, `{"base":"fixed","paths":[]}`, "criteria source has no standards path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testutil.Repo(t)
			testutil.Put(t, repo, ".devkit/config.json", tc.config)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "fixed source")
			testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
			base := testutil.Git(t, repo, "rev-parse", "HEAD")
			testutil.Put(t, repo, ".devkit/config.json", `{}`)
			testutil.Standards(t, repo, ".devkit/standards")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "valid current source")
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			must(record.Start(dir, "judge", record.StartOptions{RunID: "judge-1", Commit: head, BaseCommit: base}))
			testutil.Put(t, artifacts, "criteria/keep", "unchanged\n")
			if tc.cached != "" {
				testutil.Put(t, artifacts, "criteria/source.json", tc.cached)
			}
			before := artifactSnapshot(t, artifacts, state)
			vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "judge-1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"judge", "prepare"}, env), 1)
			testutil.Contains(t, stderr.String(), tc.want)
			testutil.Equal(t, strings.TrimSpace(out.String()), "")
			testutil.Equal(t, artifactSnapshot(t, artifacts, state), before)
			testutil.Equal(t, must(record.List(dir, "judgment")), []doc{})
			testutil.Equal(t, must(record.List(dir, "result")), []doc{})
			testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
		})
	}
}
