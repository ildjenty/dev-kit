package implement

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

func TestUncertainStepBackAtEntryPoints(t *testing.T) {
	for _, nextStep := range []string{"", "implement", "plan", "requirements", "environment"} {
		t.Run(nextStep, func(t *testing.T) {
			ctx, a := setup(t, true)
			stateDir := t.TempDir()
			dir := record.RecordDir(stateDir, "work")
			testutil.OK(t, os.MkdirAll(filepath.Dir(dir), 0755))
			testutil.OK(t, os.Rename(ctx.Dir, dir))
			vars := map[string]string{"ARTIFACTS_DIR": a, "STATE_DIR": stateDir, "WORKFLOW_ID": ctx.RunID, "BASE_BRANCH": ctx.BaseBranch}
			registry := cli.New("test", Commands())
			execute := func(mode string, want int) record.Document {
				t.Helper()
				var out, stderr bytes.Buffer
				env := cli.Env{Cwd: ctx.Cwd, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
				if code := registry.Execute([]string{"implement", mode}, env); code != want {
					t.Fatalf("%s: %d, want %d: %s", mode, code, want, stderr.String())
				}
				if want != 0 {
					return nil
				}
				var result record.Document
				testutil.OK(t, json.Unmarshal(out.Bytes(), &result))
				return result
			}
			execute("prepare", 0)
			execute("next", 0)
			execute("tests-written", 0)
			testutil.Put(t, ctx.Cwd, "result.ext", "uncommitted implementation\n")
			head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
			status := testutil.Git(t, ctx.Cwd, "status", "--porcelain=v1")
			plan := testutil.Must(os.ReadFile(filepath.Join(a, "implement/plan.json")))
			uncertain := []string{}
			if nextStep != "" {
				uncertain = []string{"C1 says cancel; tests/cancel.ext:12 expects deletion.", "requirements.json:1 retains the cancelled booking."}
			}
			var diagnosis *stepBack
			if nextStep == "plan" || nextStep == "implement" {
				name := "returns-uncertain-to-plan"
				if nextStep == "implement" {
					name = "continues-when-uncertain-needs-no-upstream-change"
				}
				var fixture record.Document
				testutil.OK(t, yaml.Unmarshal(testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/implement/fixtures/"+name+".stubs.yaml")), &fixture))
				var code codeOutput
				testutil.OK(t, json.Unmarshal(raw(fixture["code"]), &code))
				uncertain = code.Uncertain
				testutil.OK(t, json.Unmarshal(raw(fixture["uncertain-step-back"]), &diagnosis))
			}
			vars["CODE"] = string(raw(codeOutput{Uncertain: uncertain, CommitMessage: "change: allow cancellation"}))
			vars["GATE"] = string(raw(gateFixture()))
			testutil.Equal(t, execute("uncertain", 0)["needed"], nextStep != "")
			testutil.Equal(t, testutil.Must(readJSON[[]string](filepath.Join(a, "implement/uncertain.json"))), uncertain)
			if nextStep != "" {
				state := testutil.Must(os.ReadFile(filepath.Join(a, "implement/state.json")))
				execute("end", 1)
				testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(a, "implement/state.json"))), state)
				testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
				testutil.Equal(t, testutil.Git(t, ctx.Cwd, "status", "--porcelain=v1"), status)
				if diagnosis == nil {
					diagnosis = &stepBack{Next: nextStep, Premise: "C1 test interpretation", Observation: "tests/cancel.ext:12"}
				}
				vars["STEP_BACK"] = string(raw(diagnosis))
			}
			ended := execute("end", 0)
			continued := nextStep == "" || nextStep == "implement"
			testutil.Equal(t, ended["committed"], continued)
			testutil.Equal(t, ended["all_done"], continued)
			testutil.Equal(t, ended["finished"], true)
			testutil.Equal(t, execute("state", 0)["stuck"], false)
			delete(vars, "STEP_BACK")
			result := execute("finish", 0)
			switch nextStep {
			case "", "implement":
				testutil.Equal(t, result["outcome"], "pass")
			case "plan", "requirements":
				testutil.Equal(t, result["outcome"], "back")
				back := result["back"].(map[string]any)
				testutil.Equal(t, back["to"], nextStep)
				testutil.Contains(t, text(back["observation"]), strings.Join(uncertain, "\n"))
			case "environment":
				testutil.Equal(t, result["outcome"], "stop")
			}
			if !continued {
				testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
				testutil.Equal(t, testutil.Git(t, ctx.Cwd, "status", "--porcelain=v1"), status)
				testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(a, "implement/plan.json"))), plan)
				testutil.Equal(t, len(testutil.Must(record.List(dir, "plan"))), 1)
			}
		})
	}
}

func TestUncertainContinueKeepsFailingCheckGate(t *testing.T) {
	ctx, a := setup(t, true)
	testutil.Must(prepare(ctx, a))
	testutil.Must(next(a))
	gate := gateFixture()
	gate.Passed, gate.Failing = false, []string{"tests"}
	head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	ended := testutil.Must(end(ctx, a, gate, &codeOutput{Uncertain: []string{"tests/cancel.ext:12 may misread C1"}}, &stepBack{Next: "implement"}))
	testutil.Equal(t, ended["committed"], false)
	testutil.Equal(t, ended["finished"], false)
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
}
