package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestWorkflowStartsRefuseMissingCommandsBeforeRecording(t *testing.T) {
	for _, workflow := range []string{"requirements", "plan", "implement", "judge", "fix", "pr", "sync", "refine", "retro"} {
		for _, source := range []string{"yaml", "prompt"} {
			t.Run(workflow+"/"+source, func(t *testing.T) {
				repo, pack, state := testutil.Repo(t), t.TempDir(), t.TempDir()
				testutil.Put(t, pack, workflow+"/"+workflow+".yaml", "bash: devkit record start "+workflow)
				missingFile := workflow + "/commands/write.md"
				if source == "yaml" {
					missingFile = workflow + "/" + workflow + ".yaml"
				}
				testutil.Put(t, pack, missingFile, "devkit future missing")
				if source == "prompt" {
					testutil.Put(t, pack, kitinfo.StampName, `{"devkit_version":"v-old","kit_digest":"old"}`)
				}
				values := map[string]string{"BASE_BRANCH": "main", "PACK_DIR": pack, "STATE_DIR": state, "WORKFLOW_ID": "mismatched", "INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Yes"}]`, "ARTIFACTS_DIR": t.TempDir()}
				var out, stderr bytes.Buffer
				env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { value, ok := values[key]; return value, ok }}
				args := []string{"record", "start", workflow}
				if workflow == "refine" || workflow == "retro" {
					args = []string{workflow, "start"}
				}
				testutil.Equal(t, newRegistry().Execute(args, env), 0)
				var start record.StartOutput
				testutil.OK(t, json.Unmarshal(out.Bytes(), &start))
				testutil.Equal(t, start.Stopped, true)
				testutil.Contains(t, start.Reason, "devkit future missing")
				testutil.Contains(t, start.Reason, "the installed workflows and this devkit binary come from different versions")
				testutil.Contains(t, start.Reason, "run `devkit init` in the repository with the matching devkit")
				if source == "prompt" {
					testutil.Contains(t, start.Reason, "install devkit v-old")
				}
				if workflow == "requirements" || workflow == "refine" || workflow == "retro" {
					encoded := out.String()
					out.Reset()
					args = []string{workflow, "not-started", encoded}
					if workflow != "requirements" {
						args = []string{workflow, "not-started"}
						values["INPUTS_START"] = encoded
					}
					testutil.Equal(t, newRegistry().Execute(args, env), 0)
					var result map[string]any
					testutil.OK(t, json.Unmarshal(out.Bytes(), &result))
					testutil.Equal(t, result["outcome"], "not_started")
					testutil.Contains(t, out.String(), "devkit future missing")
				}
				entries := testutil.Must(os.ReadDir(state))
				testutil.Equal(t, len(entries), 0)
				testutil.Equal(t, stderr.String(), "")
			})
		}
	}
}

func TestCompatibleStartChecksOnlyItsWorkflowAndIgnoresDigestAndFixtures(t *testing.T) {
	repo, pack := testutil.Repo(t), t.TempDir()
	testutil.Put(t, repo, ".devkit/config.json", `{}`)
	testutil.Git(t, repo, "add", ".devkit/config.json")
	testutil.Git(t, repo, "commit", "-qm", "config")
	testutil.Put(t, pack, "requirements/requirements.yaml", "bash: devkit record start requirements")
	testutil.Put(t, pack, "requirements/commands/write.md", "Use `devkit requirements prepare`.")
	testutil.Put(t, pack, "requirements/fixtures/main.stubs.yaml", "devkit fixture before")
	testutil.Put(t, pack, "plan/commands/write.md", "devkit future missing")
	testutil.Put(t, pack, kitinfo.StampName, `{"devkit_version":"older","kit_digest":"different"}`)
	state := t.TempDir()
	values := map[string]string{"BASE_BRANCH": "main", "PACK_DIR": pack, "STATE_DIR": state, "WORKFLOW_ID": "compatible"}
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { value, ok := values[key]; return value, ok }}
	testutil.Equal(t, newRegistry().Execute([]string{"record", "start", "requirements"}, env), 0)
	var start record.StartOutput
	testutil.OK(t, json.Unmarshal(out.Bytes(), &start))
	testutil.Equal(t, start.Stopped, false)
	runs := testutil.Must(record.List(env.Path(start.Details), "run"))
	testutil.Equal(t, len(runs), 1)
	testutil.Equal(t, stderr.String(), "")
}
