package pr

import (
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func must[T any](v T, err error) T { return testutil.Must(v, err) }
func versions() doc                { return doc{"requirements_version": 1, "plan_version": 1} }
func judgment(run, judge, base, commit string, extra doc) doc {
	j := doc{"run_id": run, "recorded_at": "t", "judge": judge, "kind": "code", "target": doc{"requirements_version": 1, "plan_version": 1, "base": base, "commit": commit}, "criteria_commit": "c", "materials": []string{}, "observations": []string{}, "verdict": "pass", "findings": []doc{}}
	for k, v := range extra {
		j[k] = v
	}
	return j
}
func judgedAs(id, run, judge, base, commit string, extra doc) doc {
	j := judgment(run, judge, base, commit, extra)
	object(j["target"])["change_id"] = id
	return j
}
func chain(judgments []doc, head string, v doc, id string) doc {
	return verifyChain(judgments, head, v, &id)
}
func TestUnpassedUnassessedOrBlockingJudgmentStopsPR(t *testing.T) {
	blocking := doc{"id": "F1", "blocking": true, "needs_decision": false, "text": "breaks", "evidence": "a.ts:1"}
	for _, extra := range []doc{{"verdict": "fail"}, {"verdict": "not_assessed"}, {"findings": []doc{blocking}}} {
		testutil.Equal(t, chain([]doc{judgedAs("same", "j1", "scope", "m", "c", extra)}, "c", versions(), "same")["ok"], false)
	}
}
func TestOnlyLastJudgeRunOfChangeCounts(t *testing.T) {
	judgments := []doc{judgedAs("same", "j1", "scope", "m", "c", doc{"verdict": "fail"}), judgedAs("same", "j1", "security", "m", "c", doc{"verdict": "fail"}), judgedAs("same", "j2", "scope", "m", "c", nil)}
	testutil.Equal(t, chain(judgments, "c", versions(), "same")["ok"], true)
	testutil.Equal(t, chain([]doc{judgments[2], judgments[1], judgments[0]}, "c", versions(), "same")["ok"], false)
}
func TestJudgmentOnOlderRequirementsOrPlanDoesNotCount(t *testing.T) {
	for _, v := range []doc{{"requirements_version": 2, "plan_version": 1}, {"requirements_version": 1, "plan_version": 2}} {
		c := chain([]doc{judgedAs("same", "j1", "scope", "m", "c", nil)}, "c", v, "same")
		testutil.Equal(t, c["ok"], false)
		testutil.Contains(t, stringsOf(c["reasons"])[0], "not the latest")
	}
}
func TestJudgmentsMustIncludeScopeJudge(t *testing.T) {
	c := chain([]doc{judgedAs("same", "j1", "test", "m", "c", nil)}, "c", versions(), "same")
	testutil.Equal(t, c["ok"], false)
	testutil.Contains(t, stringsOf(c["reasons"])[0], "no scope judge")
}
func TestChainStartsAtJudgmentOfSameDiffAsHEAD(t *testing.T) {
	c := chain([]doc{judgedAs("same", "j1", "scope", "m", "c", nil)}, "c", versions(), "same")
	testutil.Equal(t, c["ok"], true)
	testutil.Equal(t, c["judged_commit"], "c")
}
func TestSyncMoveWithoutDiffChangeKeepsJudgment(t *testing.T) {
	c := chain([]doc{judgedAs("same", "j1", "scope", "m", "c", nil)}, "h", versions(), "same")
	testutil.Equal(t, c["ok"], true)
	testutil.Equal(t, c["judged_commit"], "c")
}
func TestChangedDiffOrMissingChangeIDDoesNotReachHEAD(t *testing.T) {
	c := chain([]doc{judgedAs("same", "j1", "scope", "m", "c", nil)}, "h", versions(), "other")
	testutil.Equal(t, c["ok"], false)
	testutil.Contains(t, stringsOf(c["reasons"])[0], "no code judgment judged the change of h (change-id other)")
	testutil.Equal(t, chain([]doc{judgment("j1", "scope", "m", "h", nil)}, "h", versions(), "same")["ok"], true)
}
func TestLastJudgmentOfSameDiffStartsChain(t *testing.T) {
	c := chain([]doc{judgedAs("same", "j1", "scope", "m", "c", nil), judgedAs("same", "j2", "scope", "m", "h", doc{"verdict": "fail"})}, "h", versions(), "same")
	testutil.Equal(t, c["ok"], false)
	testutil.Equal(t, c["judged_commit"], "h")
}
func TestChangeIDSurvivesRebaseButNotDiffChange(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string { return testutil.Git(t, repo, args...) }
	commit := func(m string) { git("commit", "-qam", m) }
	git("init", "-q", "-b", "main")
	base := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	testutil.Put(t, repo, "a.txt", base)
	git("add", ".")
	commit("base")
	git("checkout", "-q", "-b", "feature")
	testutil.Put(t, repo, "a.txt", strings.Replace(base, "10", "ten", 1))
	commit("work")
	before := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	moved := "one\n" + strings.TrimPrefix(base, "1\n")
	testutil.Put(t, repo, "a.txt", moved)
	commit("main moved")
	git("checkout", "-q", "feature")
	git("rebase", "-q", "main")
	after := git("rev-parse", "HEAD")
	g := must(gitx.New(repo, "main"))
	if before == after {
		t.Fatal("rebase did not move HEAD")
	}
	testutil.Equal(t, *must(g.ChangeID(before)), *must(g.ChangeID(after)))
	if must(g.MergeBase(before)) == must(g.MergeBase(after)) {
		t.Fatal("merge base did not move")
	}
	testutil.Put(t, repo, "a.txt", strings.Replace(moved, "10", "TEN", 1))
	commit("one more line")
	if *must(g.ChangeID(git("rev-parse", "HEAD"))) == *must(g.ChangeID(after)) {
		t.Fatal("change-id did not change")
	}
}
