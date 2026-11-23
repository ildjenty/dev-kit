package judge

import (
	"github.com/ildjenty/dev-kit/internal/cli"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestF5CodeRejectsInvalidDeferred(t *testing.T) {
	for _, tc := range []struct {
		quality string
		flags   doc
	}{
		{"security", doc{"blocking": false}},
		{"structure", doc{"blocking": true}},
		{"structure", doc{"blocking": false, "needs_decision": true}},
	} {
		tc.flags["deferred"] = true
		result := must(gather(doc{tc.quality: doc{"criteria": []doc{criterion("pass")}, "findings": []doc{finding(tc.flags)}}}, doc{"launch_" + tc.quality: true}))
		testutil.Equal(t, verdictOf(docs(result[0]["criteria"]), docs(result[0]["findings"])), "not_assessed")
	}
}

func TestF6EmptyCriteriaNeverPass(t *testing.T) {
	testutil.Equal(t, verdictOf(nil, nil), "not_assessed")
}

func TestF8ReviewOnlyNeverReturnsBack(t *testing.T) {
	for _, changes := range []doc{{"unlisted": []doc{{"path": "infra/a", "quality": "cost"}}}, {"unlisted": []doc{}}} {
		input := inputs(doc{"prepare": doc{"stopped": false, "has_requirements": false}, "changes": changes})
		object(input["findings"])["judges"] = []doc{{"judge": "scope", "criteria": []doc{criterion("pass")}, "findings": []doc{finding(doc{"needs_decision": true})}}}
		_, result := decide(input)
		testutil.Equal(t, result["outcome"], "stop")
		testutil.Equal(t, object(result["stop"])["state"], "this change needs requirements and a plan: start `requirements` describing it")
	}
}

func TestF9PrepareExportsWholeJudgedTree(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, "unchanged/peer.ext", "  peer \n\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "criteria and peer")
	dir := t.TempDir()
	must(record.Append(dir, "request", doc{"owner_words": "Review", "supplement": "", "criteria_commit": testutil.Git(t, repo, "rev-parse", "HEAD"), "run_id": "r0", "recorded_at": "t"}))
	a := t.TempDir()
	must(prepare(record.ContextData{Cwd: repo, Dir: dir, RunID: "j1", BaseBranch: "main"}, a, a))
	b, err := os.ReadFile(filepath.Join(a, "judge/tree/unchanged/peer.ext"))
	testutil.OK(t, err)
	testutil.Equal(t, string(b), "  peer \n\n")
}

func TestF10SourceCopiesAreByteExact(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, "sample.ext", "\n  before \n\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.Put(t, repo, "sample.ext", "\n  after \n\n")
	testutil.Git(t, repo, "commit", "-qam", "change")
	dir, a := t.TempDir(), t.TempDir()
	must(record.Append(dir, "request", doc{"owner_words": "Review", "supplement": "", "criteria_commit": testutil.Git(t, repo, "rev-parse", "main"), "run_id": "r0", "recorded_at": "t"}))
	must(prepare(record.ContextData{Cwd: repo, Dir: dir, RunID: "j1", BaseBranch: "main"}, a, a))
	must(writeChanges(repo, a))
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(a, "judge/source/base/sample.ext")))), "\n  before \n\n")
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(a, "judge/source/commit/sample.ext")))), "\n  after \n\n")
}

func TestF6EveryMaterialCriterionMustBePresent(t *testing.T) {
	a := t.TempDir()
	testutil.OK(t, os.MkdirAll(filepath.Join(a, "judge/materials"), 0755))
	testutil.OK(t, save(filepath.Join(a, "judge/materials/scope.json"), doc{"criteria": []string{"review/scope.md", "review/other.md"}}))
	vars := map[string]string{"ARTIFACTS_DIR": a, "CHANGES": `{"launch_scope":true}`, "JUDGE_SCOPE": `{"criteria":[{"aspect":"scope","verdict":"pass","checked":"checked","not_checked":""}],"findings":[]}`}
	env := cli.Env{Cwd: t.TempDir(), Stdout: io.Discard, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	testutil.OK(t, collect(nil, env))
	output := must(read(filepath.Join(a, "judge/findings.json")))
	criteria := docs(docs(output["judges"])[0]["criteria"])
	testutil.Equal(t, verdictOf(criteria, nil), "not_assessed")
	testutil.Contains(t, text(criteria[len(criteria)-1]["not_checked"]), "review/other.md")
}
