package gitx

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRepositoryQueriesAndRefs(t *testing.T) {
	repo := testutil.Repo(t)
	g := testutil.Must(New(repo, ""))
	testutil.Equal(t, g.Refs, []string{"main"})
	head := testutil.Must(g.Head())
	testutil.Equal(t, head, testutil.Git(t, repo, "rev-parse", "HEAD"))
	testutil.Equal(t, testutil.Must(g.Branch()), "feature/x")
	base := testutil.Git(t, repo, "rev-parse", "main")
	testutil.Equal(t, testutil.Must(g.MergeBase("")), base)
	testutil.Equal(t, testutil.Must(g.MergeBases("HEAD")), []string{base})
	testutil.Equal(t, g.IsAncestor(base, head), true)
	testutil.Equal(t, testutil.Must(g.Clean()), true)
	testutil.Equal(t, len(*testutil.Must(g.ChangeID("HEAD"))), 64)
	testutil.Put(t, repo, "untracked.txt", "untracked\n")
	testutil.Equal(t, testutil.Must(g.Clean()), false)
	testutil.Git(t, repo, "update-ref", "refs/remotes/origin/main", base)
	g = testutil.Must(New(repo, "main"))
	testutil.Equal(t, g.Refs, []string{"origin/main", "main"})
	testutil.Equal(t, testutil.Must(g.MergeBases("HEAD")), []string{base})
	_, err := New(repo, "missing")
	testutil.Fails(t, err, "neither origin/missing nor missing is a commit")
}
func TestChangeIDSurvivesCommitMovedOntoMain(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "feature.txt", "feature\n")
	testutil.Git(t, repo, "add", "feature.txt")
	testutil.Git(t, repo, "commit", "-qm", "feature")
	g := testutil.Must(New(repo, "main"))
	before := testutil.Must(g.ChangeID("HEAD"))
	testutil.Git(t, repo, "checkout", "-q", "main")
	testutil.Put(t, repo, "base.txt", "base\n")
	testutil.Git(t, repo, "add", "base.txt")
	testutil.Git(t, repo, "commit", "-qm", "advance main")
	testutil.Git(t, repo, "checkout", "-q", "feature/x")
	testutil.Git(t, repo, "rebase", "main")
	after := testutil.Must(g.ChangeID("HEAD"))
	testutil.Equal(t, *before, *after)
	testutil.Equal(t, testutil.Must(g.ChangedFiles("", "")), []File{{Status: "A", Path: "feature.txt"}})
	added, removed, err := g.DirectoryChanges("", "")
	testutil.OK(t, err)
	testutil.Equal(t, added, []string{})
	testutil.Equal(t, removed, []string{})
}
func TestUnrelatedHistoryHasNoChangeID(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "checkout", "--orphan", "unrelated")
	testutil.Git(t, repo, "commit", "--allow-empty", "-qm", "unrelated")
	g := testutil.Must(New(repo, "main"))
	testutil.Equal(t, testutil.Must(g.ChangeID("HEAD")), nil)
	_, err := g.WorkingChanges("")
	testutil.Fails(t, err, "HEAD has no merge base")
}

func TestChangeIdentityPreservesWhitespaceAndIgnoresHunkPositionsAndBacklog(t *testing.T) {
	repo := testutil.Repo(t)
	git := func(args ...string) string { return testutil.Git(t, repo, args...) }
	base := "def allowed(staff):\n    if staff:\n        return True\n    return False\n"
	testutil.Put(t, repo, "auth.py", base)
	git("add", ".")
	git("commit", "-qm", "base")
	git("branch", "-f", "main", "HEAD")
	testutil.Put(t, repo, "auth.py", base+"\nvalue = 1\n")
	git("commit", "-qam", "change")
	g := testutil.Must(New(repo, "main"))
	id := *testutil.Must(g.ChangeID("HEAD"))
	git("checkout", "-q", "main")
	testutil.Put(t, repo, "auth.py", "# header\n# moves hunk\n"+base)
	git("commit", "-qam", "move lines")
	git("checkout", "-q", "feature/x")
	git("rebase", "main")
	testutil.Equal(t, *testutil.Must(g.ChangeID("HEAD")), id)
	testutil.Put(t, repo, "auth.py", "# header\n# moves hunk\n"+base+"\nvalue  = 1\n")
	git("commit", "-qam", "whitespace")
	if *testutil.Must(g.ChangeID("HEAD")) == id {
		t.Fatal("whitespace was ignored")
	}
	testutil.Put(t, repo, "auth.py", "# header\n# moves hunk\n"+base+"\nvalue = 1\n")
	git("commit", "-qam", "restore whitespace")
	testutil.Put(t, repo, ".devkit/backlog/feature%2Fx.json", `{"deferred":[],"carried":[],"mechanical":[],"accepted_objections":[]}`)
	git("add", ".")
	git("commit", "-qm", "backlog")
	testutil.Equal(t, *testutil.Must(g.ChangeID("HEAD")), id)

}

func TestF3OnlyValidCurrentBranchBacklogIsExcluded(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		excluded   bool
	}{
		{".devkit/backlog/feature%2Fx.json", `{"deferred":[],"carried":[],"mechanical":[],"accepted_objections":[]}`, true},
		{".devkit/backlog/feature%2Fx.json", `{}`, false},
		{".devkit/backlog/feature%2Fx.json", `{"deferred":[{}],"carried":[],"mechanical":[],"accepted_objections":[]}`, false},
		{".devkit/backlog/other.json", `{"deferred":[],"carried":[],"mechanical":[],"accepted_objections":[]}`, false},
		{".devkit/backlog/runtime.sh", "echo allowed\n", false},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			repo := testutil.Repo(t)
			g := testutil.Must(New(repo, "main"))
			before := *testutil.Must(g.ChangeID("HEAD"))
			testutil.Put(t, repo, tc.path, tc.body)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "backlog")
			after := *testutil.Must(g.ChangeID("HEAD"))
			testutil.Equal(t, before == after, tc.excluded)
		})
	}
}

func TestF9ArchiveKeepsIgnoredAndSubstitutedBlobsAndRemovesStaleFiles(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".gitattributes", "ignored.ext export-ignore\nsubstituted.ext export-subst\n")
	testutil.Put(t, repo, "ignored.ext", "\n  ignored \n\n")
	testutil.Put(t, repo, "substituted.ext", "$Format:%H$\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "archive attributes")
	g := testutil.Must(New(repo, "main"))
	out := t.TempDir()
	testutil.Put(t, out, "stale.ext", "stale")
	testutil.OK(t, g.ExportTree("HEAD", out))
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(out, "ignored.ext")))), "\n  ignored \n\n")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(out, "substituted.ext")))), "$Format:%H$\n")
	_, err := os.Stat(filepath.Join(out, "stale.ext"))
	testutil.Equal(t, os.IsNotExist(err), true)
}

func TestUnitMarkersAreOrdinaryJudgedChanges(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		excluded   bool
	}{
		{"U-abcdef.done.json", `{"unit":"U-abcdef","branch":"feature/x"}`, false},
		{"U-abcdef.done.json", `{}`, false},
		{"U-abcdef.done.json", `{"unit":"U-abcdef","branch":"other"}`, false},
		{"U-abcdef.done.json", `{"unit":"U-fedcba","branch":"feature/x"}`, false},
		{"U-fedcba.done.json", `{"unit":"U-fedcba","branch":"feature/x"}`, false},
		{"U-abcdef.md", "Changed unit\n", false},
		{"runtime.sh", "echo changed\n", false},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			repo, records := testutil.Repo(t), t.TempDir()
			testutil.Put(t, records, "request/1.json", `{"unit":"U-abcdef"}`)
			testutil.Put(t, repo, ".devkit/config.json", `{"units":"work/units"}`)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "configuration")
			g := testutil.Must(New(repo, "main", records))
			before := *testutil.Must(g.ChangeID("HEAD"))
			path := "work/units/" + tc.path
			testutil.Put(t, repo, path, tc.body)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "marker or other unit file")
			testutil.Equal(t, before == *testutil.Must(g.ChangeID("HEAD")), tc.excluded)
			changes := testutil.Must(g.Changes("", "HEAD"))
			testutil.Equal(t, slices.Contains(changes.Excluded, path), tc.excluded)
			if tc.excluded {
				g.RecordDir = ""
				t.Setenv("STATE_DIR", "")
				testutil.Equal(t, before == *testutil.Must(g.ChangeID("HEAD")), false)
				testutil.Put(t, repo, path, `{"unit":"U-abcdef","branch":"feature/x","extra":true}`)
				g.RecordDir = records
				working := testutil.Must(g.WorkingChanges(""))
				testutil.Equal(t, slices.Contains(working.Excluded, path), false)
			}
		})
	}
}
