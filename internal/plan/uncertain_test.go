package plan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func TestDetailUncertainReachesStructureQuestionAtEntryPoints(t *testing.T) {
	var fixture document
	testutil.OK(t, yaml.Unmarshal(must(os.ReadFile("../../kit/.archon/workflows/dev/plan/fixtures/asks-about-detail-uncertain.stubs.yaml")), &fixture))
	for _, tc := range []struct {
		name      string
		uncertain []string
		valid     bool
	}{{"empty", []string{}, true}, {"structure", stringsOf(object(fixture["write"])["uncertain"]), true}, {"invalid-detail", stringsOf(object(fixture["write"])["uncertain"]), false}} {
		t.Run(tc.name, func(t *testing.T) {
			uncertain := tc.uncertain
			ctx, a := startStructure(t)
			ctx.Branch = testutil.Git(t, ctx.Cwd, "branch", "--show-current")
			request(ctx.Dir)
			requirements(ctx.Dir, "r1")
			run(ctx.Dir, ctx.RunID)
			d := draft()
			if !tc.valid {
				d["increments"] = []document{}
			}
			testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), d))
			testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "detail"))["valid"], tc.valid)
			head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
			before := testutil.Git(t, ctx.Cwd, "status", "--porcelain=v1")
			fixed := must(os.ReadFile(filepath.Join(a, "plan/structure.json")))
			stateDir := t.TempDir()
			dir := record.RecordDir(stateDir, ctx.Branch)
			testutil.OK(t, os.MkdirAll(filepath.Dir(dir), 0755))
			testutil.OK(t, os.Rename(ctx.Dir, dir))
			vars := map[string]string{"ARTIFACTS_DIR": a, "STATE_DIR": stateDir, "WORKFLOW_ID": ctx.RunID, "BASE_BRANCH": ctx.BaseBranch, "INPUTS_UNCERTAIN": string(must(json.Marshal(uncertain))), "INPUTS_STRUCTURE_QUESTION": "May work proceed with this structure?", "INPUTS_DELIVERY": string(must(json.Marshal(judgment(nil))))}
			registry := cli.New("test", Commands())
			execute := func(mode string) document {
				t.Helper()
				var out, stderr bytes.Buffer
				env := cli.Env{Cwd: ctx.Cwd, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
				if code := registry.Execute([]string{"plan", mode}, env); code != 0 {
					t.Fatalf("%s: %d: %s", mode, code, stderr.String())
				}
				var result document
				testutil.OK(t, json.Unmarshal(out.Bytes(), &result))
				return result
			}
			testutil.Equal(t, execute("settle")["done"], true)
			state := must(read(filepath.Join(rounds(a), "latest.json")))
			testutil.Equal(t, stringsOf(state["uncertain"]), uncertain)
			result := execute("finish")
			if len(uncertain) == 0 {
				testutil.Equal(t, result["outcome"], "pass")
				return
			}
			testutil.Equal(t, result["outcome"], "question")
			questions := must(record.ListKeyed(dir, "question"))
			testutil.Equal(t, len(questions), 1)
			testutil.Equal(t, questions[0]["text"], "May work proceed?\n"+strings.Join(uncertain, "\n"))
			testutil.Equal(t, stringsOf(questions[0]["next_workflows"]), []string{"plan"})
			testutil.Equal(t, object(questions[0]["target"])["kind"], "change")
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "status", "--porcelain=v1"), before)
			testutil.Equal(t, must(os.ReadFile(filepath.Join(a, "plan/structure.json"))), fixed)
			_, err := os.Stat(filepath.Join(ctx.Cwd, "adapter/new.ext"))
			if !os.IsNotExist(err) {
				t.Fatalf("the reported structure change was applied: %v", err)
			}
			testutil.Equal(t, must(record.Latest(dir, "result"))["outcome"], "question")
		})
	}
}

func TestSettleRejectsMalformedUncertainBeforeWriting(t *testing.T) {
	_, a := startStructure(t)
	before := must(os.ReadFile(filepath.Join(rounds(a), "latest.json")))
	for _, value := range []string{`[`, `[{}]`, `"uncertain"`} {
		var out, stderr bytes.Buffer
		vars := map[string]string{"ARTIFACTS_DIR": a, "INPUTS_UNCERTAIN": value}
		env := cli.Env{Cwd: t.TempDir(), Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
		testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"plan", "settle"}, env), 1)
		testutil.Equal(t, out.Len(), 0)
		testutil.Equal(t, must(os.ReadFile(filepath.Join(rounds(a), "latest.json"))), before)
	}
}
