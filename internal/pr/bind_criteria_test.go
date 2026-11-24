package pr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestFactsUsesFixedMaterialsAfterStandardsRelocation(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "reused"}[reuse], func(t *testing.T) {
			repo := testutil.Repo(t)
			testutil.Put(t, repo, ".devkit/config.json", `{"language":"fr","standards":"rules","decision_paths":[]}`)
			testutil.Standards(t, repo, "rules")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "Fixed standards")
			testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
			base := testutil.Git(t, repo, "rev-parse", "HEAD")
			mapping := must(concerns.Load(filepath.Join(repo, "rules")))
			authority := must(os.ReadFile(filepath.Join(repo, "rules/authority.md")))
			state, artifacts := t.TempDir(), t.TempDir()
			dir := record.RecordDir(state, "feature/x")
			must(record.Append(dir, "request", doc{"run_id": "r1", "recorded_at": "t", "owner_words": "Relocate standards", "supplement": "", "criteria_commit": base}))
			if reuse {
				must(criteria.CopyCriteria(criteria.Options{Root: repo, Base: base, Dir: filepath.Join(artifacts, "pr/criteria")}))
			}
			testutil.Git(t, repo, "mv", "rules", "policy")
			testutil.Put(t, repo, "policy/authority.md", "Current authority\n")
			testutil.Put(t, repo, "policy/concerns.json", `{}`)
			testutil.Put(t, repo, ".devkit/config.json", `{"language":"de","standards":"policy","decision_paths":[]}`)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "Relocate standards")
			head := testutil.Git(t, repo, "rev-parse", "HEAD")
			addRun(dir, "r1", "requirements")
			must(record.Append(dir, "requirements", doc{"request": 1, "run_id": "r1", "recorded_at": "t", "goal": "Relocate standards", "completion_conditions": []doc{{"id": "C1", "text": "Standards are relocated"}}, "constraints": []string{}, "open_points": []doc{}}))
			must(record.WriteResult(dir, "requirements", doc{"outcome": "pass"}, record.ResultOptions{RunID: "r1", Base: base, Commit: head}))
			addRun(dir, "p1", "plan")
			must(record.Append(dir, "plan", doc{"requirements_version": 1, "run_id": "p1", "recorded_at": "t", "skeleton_commit": head, "decisions": []doc{}, "increments": []doc{}, "tests": []doc{}, "kept_behaviors": []doc{}}))
			must(record.WriteResult(dir, "plan", doc{"outcome": "pass"}, record.ResultOptions{RunID: "p1", Base: base, Commit: head}))
			addRun(dir, "j1", "judge")
			g := must(gitx.New(repo, "main"))
			must(record.Append(dir, "judgment", doc{"run_id": "j1", "recorded_at": "t", "kind": "code", "judge": "scope", "target": doc{"requirements_version": 1, "plan_version": 1, "base": base, "commit": head, "change_id": must(g.ChangeID(head))}, "criteria_commit": base, "materials": []string{}, "observations": []string{}, "verdict": "pass", "findings": []doc{}}))
			addRun(dir, "pr1", "pr")
			display := must(filepath.Rel(repo, artifacts))
			vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "pr1", "ARTIFACTS_DIR": display, "BASE_BRANCH": "main"}
			testutil.Equal(t, runCLI(t, repo, vars, "facts")["ready"], true)
			facts := must(read(filepath.Join(artifacts, "pr/facts.json")))
			fixed := filepath.Join(display, "pr/criteria/rules")
			testutil.Equal(t, facts["language"], "de")
			testutil.Equal(t, facts["standards"], "rules")
			testutil.Equal(t, facts["criteria_commit"], base)
			testutil.Equal(t, facts["criteria_folder"], fixed)
			testutil.Equal(t, facts["writer_materials"], mapping.Materials(fixed, "writer:pr-body", "code"))
			testutil.Equal(t, facts["judge_materials"], mapping.Materials(fixed, "judge:pr-body", "code"))
			testutil.Equal(t, facts["criteria"], mapping.Materials(fixed, "judge:pr-body", "code").Criteria)
			testutil.Equal(t, must(os.ReadFile(filepath.Join(artifacts, "pr/criteria/rules/authority.md"))), authority)
			testutil.Equal(t, must(read(filepath.Join(artifacts, "pr/criteria/source.json"))), doc{"base": base, "paths": []string{"rules"}})
		})
	}
}
