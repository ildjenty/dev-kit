package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestLimitsRequirePositiveIntegers(t *testing.T) {
	for _, key := range []string{"passes", "fixes", "restarts"} {
		for _, n := range []int{0, -1} {
			_, err := Parse([]byte(fmt.Sprintf(`{"limits":{"%s":%d}}`, key, n)))
			testutil.Fails(t, err, "limits."+key)
			testutil.Fails(t, err, "at least 1")
		}
		c := testutil.Must(Parse([]byte(fmt.Sprintf(`{"limits":{"%s":1}}`, key))))
		limits := map[string]int{"passes": c.Limits.Passes, "fixes": c.Limits.Fixes, "restarts": c.Limits.Restarts}
		testutil.Equal(t, limits[key], 1)
	}
	testutil.Equal(t, testutil.Must(Parse([]byte(`{}`))).Limits, Limits{Passes: 3, Fixes: 3, Restarts: 3})
}

func TestChecksAcrossBaseRejectsUnreadableGitObject(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"checks":[{"name":"base","command":"true","timeout_seconds":1}]}`)
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base checks")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	blob := testutil.Git(t, repo, "rev-parse", "HEAD:.devkit/config.json")
	testutil.OK(t, os.Remove(filepath.Join(repo, ".git/objects", blob[:2], blob[2:])))
	testutil.Put(t, repo, ".devkit/config.json", `{}`)
	_, err := ChecksAcrossBase(repo, "main")
	testutil.Fails(t, err, ".devkit/config.json")
}

func TestChecksAcrossBaseAllowsMissingBaseConfig(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Equal(t, testutil.Must(ChecksAcrossBase(repo, "main")), []Check{})
}
