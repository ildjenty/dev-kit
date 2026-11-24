package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestConflictResolverUsesFixedMaterialsAfterStandardsRelocation(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "reused"}[reuse], func(t *testing.T) {
			repo, _, _ := conflictRepo(t)
			testutil.Git(t, repo, "checkout", "-q", "main")
			testutil.Standards(t, repo, "rules")
			testutil.Put(t, repo, ".devkit/config.json", `{"language":"fr","standards":"rules"}`)
			testutil.Git(t, repo, "add", "-A")
			testutil.Git(t, repo, "commit", "-qm", "Fixed standards")
			base := testutil.Git(t, repo, "rev-parse", "HEAD")
			mapping := must(concerns.Load(filepath.Join(repo, "rules")))
			authority := must(os.ReadFile(filepath.Join(repo, "rules/authority.md")))
			testutil.Git(t, repo, "checkout", "-q", "feature/x")
			testutil.Git(t, repo, "cherry-pick", base)
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			started := must(record.Start(dir, "sync", record.StartOptions{RunID: "sync1", BaseCommit: base, Commit: testutil.Git(t, repo, "rev-parse", "HEAD")}))
			testutil.Equal(t, started.Stopped, false)
			if reuse {
				must(criteria.CopyCriteria(criteria.Options{Root: repo, Base: base, Dir: filepath.Join(artifacts, "criteria")}))
			}
			testutil.Git(t, repo, "mv", "rules", "policy")
			testutil.Put(t, repo, "policy/authority.md", "Current authority\n")
			testutil.Put(t, repo, "policy/concerns.json", `{}`)
			testutil.Put(t, repo, ".devkit/config.json", `{"language":"de","standards":"policy"}`)
			testutil.Git(t, repo, "add", "-A")
			testutil.Git(t, repo, "commit", "-qm", "Relocate standards")
			display := must(filepath.Rel(repo, artifacts))
			vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "sync1", "ARTIFACTS_DIR": display, "BASE_BRANCH": "main"}
			testutil.Equal(t, runCLI(t, repo, vars, "start")["state"], "conflict")
			for _, name := range []string{"input", "conflict"} {
				var input record.Document
				testutil.OK(t, readJSON(filepath.Join(artifacts, "sync", name+".json"), &input))
				testutil.Equal(t, input["language"], "de")
				testutil.Equal(t, input["standards"], "rules")
				testutil.Equal(t, input["writer_materials"], mapping.Materials(filepath.Join(display, "criteria/rules"), "writer:sync", "code"))
			}
			testutil.Equal(t, must(os.ReadFile(filepath.Join(artifacts, "criteria/rules/authority.md"))), authority)
			var source criteria.Source
			testutil.OK(t, readJSON(filepath.Join(artifacts, "criteria/source.json"), &source))
			testutil.Equal(t, source, criteria.Source{Base: base, Paths: []string{"rules"}})
		})
	}
}
