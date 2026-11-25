package criteria

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestCriteriaCLIUsesFixedConfigurationAfterRelocation(t *testing.T) {
	for _, standards := range []string{".devkit/standards", "original-policy"} {
		t.Run(standards, func(t *testing.T) {
			repo := testutil.Repo(t)
			if standards != ".devkit/standards" {
				testutil.Put(t, repo, ".devkit/config.json", `{"standards":"original-policy"}`)
			}
			testutil.Put(t, repo, filepath.Join(standards, "standard.md"), "fixed\n")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "fixed criteria")
			base := testutil.Git(t, repo, "rev-parse", "HEAD")
			state := t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			testutil.Must(record.Append(dir, "request", record.Document{"owner_words": "Request", "supplement": "", "criteria_commit": base, "run_id": "r0", "recorded_at": "t"}))
			request := testutil.Must(record.List(dir, "request"))
			artifacts := t.TempDir()
			vars := map[string]string{"BASE_BRANCH": "main", "STATE_DIR": state, "ARTIFACTS_DIR": artifacts, "WORKFLOW_ID": "r1"}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}
			run := func() Source {
				t.Helper()
				out.Reset()
				stderr.Reset()
				testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"criteria"}, env), 0)
				var source Source
				testutil.OK(t, json.Unmarshal(out.Bytes(), &source))
				testutil.Equal(t, source.Base, base)
				testutil.Equal(t, source.Paths, []string{standards})
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(source.Dir, standards, "standard.md")))), "fixed\n")
				return source
			}
			original := run()
			saved := testutil.Must(os.ReadFile(filepath.Join(original.Dir, "source.json")))
			testutil.Git(t, repo, "mv", standards, "policy")
			testutil.Put(t, repo, ".devkit/config.json", `{"standards":"policy"}`)
			testutil.Put(t, repo, "policy/standard.md", "current\n")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "relocate criteria")
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			testutil.Equal(t, run(), original)
			testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(original.Dir, "source.json"))), saved)
			vars["ARTIFACTS_DIR"] = t.TempDir()
			run()
			testutil.Put(t, repo, ".devkit/config.json", "invalid current config")
			vars["ARTIFACTS_DIR"] = t.TempDir()
			run()
			testutil.Equal(t, testutil.Must(record.List(dir, "request")), request)
			testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(repo, "policy/standard.md")))), "current\n")
		})
	}
}

func TestCriteriaCLIRejectsInvalidFixedSourceWithoutWriting(t *testing.T) {
	for _, tc := range []struct{ name, config, base, want string }{
		{"invalid config", `{"standards":123}`, "", "invalid .devkit/config.json"},
		{"missing standards", `{"standards":"missing"}`, "", "missing does not exist at"},
		{"invalid commit", `{}`, "0000000000000000000000000000000000000000", "0000000000000000000000000000000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testutil.Repo(t)
			testutil.Put(t, repo, ".devkit/config.json", tc.config)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "fixed source")
			base := tc.base
			if base == "" {
				base = testutil.Git(t, repo, "rev-parse", "HEAD")
			}
			testutil.Put(t, repo, ".devkit/config.json", `{}`)
			testutil.Put(t, repo, ".devkit/standards/standard.md", "current\n")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "valid current source")
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			testutil.Must(record.Append(dir, "request", record.Document{"owner_words": "Request", "supplement": "", "criteria_commit": base, "run_id": "r0", "recorded_at": "t"}))
			request := testutil.Must(record.List(dir, "request"))
			testutil.Put(t, artifacts, "criteria/keep", "unchanged\n")
			vars := map[string]string{"BASE_BRANCH": "main", "STATE_DIR": state, "ARTIFACTS_DIR": artifacts, "WORKFLOW_ID": "r1"}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"criteria"}, env), 1)
			testutil.Contains(t, stderr.String(), tc.want)
			testutil.Equal(t, out.Len(), 0)
			entries := testutil.Must(os.ReadDir(filepath.Join(artifacts, "criteria")))
			testutil.Equal(t, len(entries), 1)
			testutil.Equal(t, entries[0].Name(), "keep")
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "criteria/keep")))), "unchanged\n")
			testutil.Equal(t, testutil.Must(record.List(dir, "request")), request)
			testutil.Equal(t, testutil.Must(record.List(dir, "result")), []record.Document{})
			testutil.Equal(t, testutil.Must(record.List(dir, "judgment")), []record.Document{})
			testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
		})
	}
}
