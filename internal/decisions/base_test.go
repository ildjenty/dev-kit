package decisions

import (
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestBaseCheckUsesRemoteConfigurationAndDecisionText(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"decisions":"choices"}`)
	testutil.Put(t, repo, "choices/D-abcdef.md", "---\nid: D-abcdef\ntitle: Old choice\nstatus: superseded\n---\nOld body.\n")
	testutil.Put(t, repo, "choices/D-fedcba.md", "---\nid: D-fedcba\ntitle: New choice\nstatus: agreed\nsupersedes: [D-abcdef]\n---\nNew body.\n")
	testutil.Put(t, repo, "choices/D-123456.md", "---\nid: D-123456\nstatus: agreed\n---\nUnchanged.\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base decisions")
	testutil.Git(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	testutil.Put(t, repo, ".devkit/config.json", `{"decisions":"wrong"}`)
	testutil.Put(t, repo, "choices/D-abcdef.md", "---\nstatus: agreed\n---\nCheckout still uses the old decision.\n")
	check := testutil.Must(CheckBase(repo, "main", []string{"D-abcdef", "D-123456", "D-654321"}))
	testutil.Equal(t, check.Base, "origin/main")
	testutil.Equal(t, len(check.Superseded), 1)
	old := check.Superseded[0]
	testutil.Equal(t, old.DecisionText, DecisionText{"D-abcdef", "Old choice", "Old body."})
	testutil.Equal(t, old.SupersededBy, []string{"D-fedcba"})
	testutil.Equal(t, old.Successors, []DecisionText{{"D-fedcba", "New choice", "New body."}})
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.Git(t, repo, "update-ref", "-d", "refs/remotes/origin/main")
	check = testutil.Must(CheckBase(repo, "main", []string{"D-abcdef"}))
	testutil.Equal(t, check.Base, "main")
	testutil.Equal(t, len(check.Superseded), 1)
}

func TestBaseCheckDefaultFolderUnchangedAndMissingRef(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "---\nstatus: agreed\n---\nStill agreed.\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "agreed")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	check := testutil.Must(CheckBase(repo, "", []string{"D-abcdef"}))
	testutil.Equal(t, len(check.Superseded), 0)
	testutil.Equal(t, check.Note, "")
	check = testutil.Must(CheckBase(repo, "missing", []string{"D-abcdef"}))
	testutil.Equal(t, len(check.Superseded), 0)
	testutil.Contains(t, check.Note, "decision check skipped")
	testutil.Contains(t, check.Note, "origin/missing")
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "missing front matter")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "invalid")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	_, err := CheckBase(repo, "main", []string{"D-abcdef"})
	testutil.Fails(t, err, "missing front matter")
}
