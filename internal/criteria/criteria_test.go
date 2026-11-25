package criteria

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
)

func TestCriteriaPinnedToCommitAndReused(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/standards/standard.md", "original\n")
	testutil.Git(t, repo, "add", ".devkit/standards")
	testutil.Git(t, repo, "commit", "-qm", "criteria")
	base := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Put(t, repo, ".devkit/standards/standard.md", "changed\n")
	dir := filepath.Join(t.TempDir(), "criteria")
	o := Options{repo, base, dir}
	source := testutil.Must(CopyCriteria(o))
	testutil.Equal(t, source, Source{dir, base, []string{".devkit/standards"}})
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/standards/standard.md")))), "original\n")
	sourceBytes := testutil.Must(os.ReadFile(filepath.Join(dir, "source.json")))
	testutil.Equal(t, strings.Contains(string(sourceBytes), `"dir"`), false)
	testutil.Put(t, dir, ".devkit/standards/standard.md", "kept\n")
	o.Base = "invalid"
	testutil.Equal(t, testutil.Must(CopyCriteria(o)), source)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, ".devkit/standards/standard.md")))), "kept\n")
}
func TestCriteriaConfiguredStandardsAndMissing(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"standards":"policy"}`)
	testutil.Git(t, repo, "add", ".devkit/config.json")
	testutil.Git(t, repo, "commit", "-qm", "configure policy")
	dir := t.TempDir()
	_, err := CopyCriteria(Options{repo, "HEAD", dir})
	testutil.Fails(t, err, "policy does not exist at HEAD")
	testutil.Put(t, repo, "policy/a.md", "policy")
	testutil.Git(t, repo, "add", "policy")
	testutil.Git(t, repo, "commit", "-qm", "policy")
	testutil.Equal(t, testutil.Must(CopyCriteria(Options{repo, "HEAD", dir})).Paths, []string{"policy"})
}
func TestCriteriaCLI(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/standards/standard.md", "standard")
	testutil.Git(t, repo, "add", ".devkit/standards")
	testutil.Git(t, repo, "commit", "-qm", "standard")
	commit := testutil.Git(t, repo, "rev-parse", "HEAD")
	state := t.TempDir()
	artifacts := t.TempDir()
	testutil.Must(record.Append(record.RecordDir(state, "feature/x"), "request", record.Document{"owner_words": "Request", "supplement": "", "criteria_commit": commit, "run_id": "r0", "recorded_at": "t"}))
	vars := map[string]string{"BASE_BRANCH": "main", "STATE_DIR": state, "ARTIFACTS_DIR": artifacts, "WORKFLOW_ID": "r1"}
	var out, stderr bytes.Buffer
	r := cli.New("test", Commands())
	testutil.Equal(t, r.Execute([]string{"criteria"}, cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}), 0)
	var source Source
	testutil.OK(t, json.Unmarshal(out.Bytes(), &source))
	testutil.Equal(t, source.Base, commit)
}

func TestCriteriaCLIPreservesRelativePaths(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/standards/standard.md", "standard")
	testutil.Git(t, repo, "add", ".devkit/standards")
	testutil.Git(t, repo, "commit", "-qm", "standard")
	commit := testutil.Git(t, repo, "rev-parse", "HEAD")
	state := "state"
	testutil.Must(record.Append(filepath.Join(repo, record.RecordDir(state, "feature/x")), "request", record.Document{"owner_words": "Request", "supplement": "", "criteria_commit": commit, "run_id": "r0", "recorded_at": "t"}))
	vars := map[string]string{"BASE_BRANCH": "main", "STATE_DIR": state, "ARTIFACTS_DIR": "artifacts", "WORKFLOW_ID": "r1"}
	var out, stderr bytes.Buffer
	code := cli.New("test", Commands()).Execute([]string{"criteria"}, cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { value, ok := vars[key]; return value, ok }})
	testutil.Equal(t, code, 0)
	var source Source
	testutil.OK(t, json.Unmarshal(out.Bytes(), &source))
	testutil.Equal(t, source.Dir, filepath.Join("artifacts", "criteria"))
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(repo, source.Dir, ".devkit/standards/standard.md")))), "standard")
}
