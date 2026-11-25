package requirements

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func bindingSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	testutil.OK(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content := ""
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content = string(data)
		}
		files[must(filepath.Rel(root, path))] = info.Mode().String() + ":" + content
		return nil
	}))
	return files
}

func TestFinishCLIRejectsUnboundRoundsBeforeWriting(t *testing.T) {
	for _, change := range []string{"new-request", "new-supplement", "same-input", "other-run", "other-run-prepare", "request-before-round", "request-after-round", "request-boundary", "fractional-request", "legacy-round", "missing-input", "malformed-input", "invalid-draft", "question-draft"} {
		t.Run(change, func(t *testing.T) {
			repo, state, artifacts := testutil.Repo(t), t.TempDir(), t.TempDir()
			testutil.Standards(t, repo, ".devkit/standards")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "standards")
			testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
			dir := record.RecordDir(state, "feature/x")
			run(dir, "r1")
			draft := requirements()
			if change == "invalid-draft" {
				draft["goal"] = ""
			}
			if change == "question-draft" {
				draft["open_points"] = []document{{"id": "O1", "text": "Which filter should be used?"}}
			}
			vars := map[string]string{"BASE_BRANCH": "main", "STATE_DIR": state, "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": artifacts, "INPUTS_OWNER_WORDS": "Filter the list", "INPUTS_REQUIREMENTS": string(must(json.Marshal(draft))), "INPUTS_JUDGMENT": string(must(json.Marshal(judgment())))}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			registry := cli.New("test", Commands())
			execute := func(mode string) {
				t.Helper()
				out.Reset()
				stderr.Reset()
				if code := registry.Execute([]string{"requirements", mode}, env); code != 0 {
					t.Fatalf("%s failed: %s", mode, stderr.String())
				}
			}
			execute("prepare")
			inputFile := filepath.Join(artifacts, "requirements", "input.json")
			initial := must(read(inputFile))
			execute("round")
			latest := filepath.Join(rounds(artifacts), "latest.json")
			entry := must(read(latest))
			for _, key := range []string{"input_id", "run_id", "request"} {
				testutil.Equal(t, entry[key], initial[key])
			}
			delete(vars, "INPUTS_OWNER_WORDS")
			switch change {
			case "new-request", "invalid-draft", "question-draft":
				vars["INPUTS_OWNER_WORDS"] = "New request never read by writer or judge"
				execute("prepare")
			case "new-supplement":
				vars["INPUTS_SUPPLEMENT"] = "Additional repository fact"
				execute("prepare")
			case "same-input":
				execute("prepare")
				testutil.Equal(t, must(read(inputFile))["request"], initial["request"])
			case "other-run", "other-run-prepare":
				run(dir, "r2")
				vars["WORKFLOW_ID"] = "r2"
				if change == "other-run-prepare" {
					execute("prepare")
				}
			case "request-before-round", "request-after-round":
				request(dir)
				if change == "request-before-round" {
					execute("round")
				}
			case "request-boundary":
				entry["request"] = 2
				testutil.OK(t, write(latest, entry))
			case "fractional-request":
				entry["request"] = 1.5
				testutil.OK(t, write(latest, entry))
				input := must(read(inputFile))
				input["request"] = 1.5
				testutil.OK(t, write(inputFile, input))
			case "legacy-round":
				testutil.OK(t, write(latest, strip(entry, "input_id", "run_id", "request")))
			case "missing-input":
				testutil.OK(t, os.Remove(inputFile))
			case "malformed-input":
				testutil.OK(t, os.WriteFile(inputFile, []byte("{"), 0644))
			}
			records, outputs := bindingSnapshot(t, state), bindingSnapshot(t, artifacts)
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			index := must(os.ReadFile(filepath.Join(repo, ".git", "index")))
			out.Reset()
			stderr.Reset()
			testutil.Equal(t, registry.Execute([]string{"requirements", "finish"}, env), 1)
			testutil.Equal(t, out.String(), "")
			if stderr.Len() == 0 {
				t.Fatal("finish rejection was not reported")
			}
			testutil.Equal(t, bindingSnapshot(t, state), records)
			testutil.Equal(t, bindingSnapshot(t, artifacts), outputs)
			testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
			testutil.Equal(t, must(os.ReadFile(filepath.Join(repo, ".git", "index"))), index)
			delete(vars, "INPUTS_OWNER_WORDS")
			delete(vars, "INPUTS_SUPPLEMENT")
			vars["INPUTS_REQUIREMENTS"] = string(must(json.Marshal(requirements())))
			execute("prepare")
			execute("round")
			execute("finish")
			testutil.Contains(t, out.String(), `"outcome":"pass"`)
			saved := must(record.Latest(dir, "requirements"))
			testutil.Equal(t, saved["request"], must(read(inputFile))["request"])
			testutil.Equal(t, saved["run_id"], vars["WORKFLOW_ID"])
			testutil.Equal(t, len(must(record.List(dir, "requirements"))), 1)
			testutil.Equal(t, len(must(record.List(dir, "judgment"))), 1)
			testutil.Equal(t, len(must(record.List(dir, "result"))), 1)
		})
	}
}

func TestRoundCLIRequiresPreparedInputBeforeWriting(t *testing.T) {
	for _, change := range []string{"missing", "malformed", "legacy", "missing-run", "missing-request", "fractional-request"} {
		t.Run(change, func(t *testing.T) {
			artifacts := t.TempDir()
			input := document{"input_id": "input1", "run_id": "r1", "request": 1}
			file := filepath.Join(artifacts, "requirements", "input.json")
			switch change {
			case "malformed":
				testutil.Put(t, artifacts, "requirements/input.json", "{")
			case "legacy":
				testutil.OK(t, write(file, document{"requests": []document{{"number": 1, "owner_words": "Request"}}}))
			case "missing-run":
				testutil.OK(t, write(file, strip(input, "run_id")))
			case "missing-request":
				testutil.OK(t, write(file, strip(input, "request")))
			case "fractional-request":
				input["request"] = 1.5
				testutil.OK(t, write(file, input))
			}
			before := bindingSnapshot(t, artifacts)
			vars := map[string]string{"ARTIFACTS_DIR": artifacts, "INPUTS_REQUIREMENTS": string(must(json.Marshal(requirements()))), "INPUTS_JUDGMENT": string(must(json.Marshal(judgment())))}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: artifacts, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"requirements", "round"}, env), 1)
			testutil.Equal(t, out.String(), "")
			if stderr.Len() == 0 {
				t.Fatal("round rejection was not reported")
			}
			testutil.Equal(t, bindingSnapshot(t, artifacts), before)
		})
	}
}

func TestFinishPreservesRequirementsRecordGuards(t *testing.T) {
	for _, field := range []string{"version", "request", "run_id", "recorded_at", "unstarted"} {
		t.Run(field, func(t *testing.T) {
			artifacts, dir := t.TempDir(), t.TempDir()
			roundInput(t, artifacts)
			request(dir)
			draft := requirements()
			if field != "unstarted" {
				run(dir, "r1")
				if field == "version" || field == "request" {
					draft[field] = 1
				} else {
					draft[field] = "supplied"
				}
			}
			must(round(artifacts, draft, judgment()))
			before, outputs := bindingSnapshot(t, dir), bindingSnapshot(t, artifacts)
			_, err := finish(record.ContextData{Dir: dir, RunID: "r1"}, artifacts, record.ResultOptions{Base: strings.Repeat("a", 40), Commit: strings.Repeat("b", 40), Now: "t"})
			if field == "unstarted" {
				testutil.Fails(t, err, "did not pass the start conditions")
			} else {
				testutil.Fails(t, err, "the record fills "+field)
			}
			testutil.Equal(t, bindingSnapshot(t, dir), before)
			testutil.Equal(t, bindingSnapshot(t, artifacts), outputs)
		})
	}
}
