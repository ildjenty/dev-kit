package status

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestStatusShowsRequestIssueNumberAndURL(t *testing.T) {
	repo, state := testutil.Repo(t), t.TempDir()
	gh := testutil.IssueCLI(t, "OPEN", "devkit:unit", "")
	dir := branch(t, filepath.Join(state, "records"), "feature/x")
	write(t, dir, "request", 1, map[string]any{"issue": 42})
	run1(t, dir, "plan", 1, map[string]any{"outcome": "pass"})
	var out bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &out, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) {
		value, ok := map[string]string{"STATE_DIR": state, "DEVKIT_GH": gh}[k]
		return value, ok
	}}
	testutil.OK(t, run(nil, env))
	testutil.Contains(t, out.String(), "issue: #42 https://github.example/team/repo/issues/42")
	out.Reset()
	testutil.OK(t, run([]string{"--json"}, env))
	var got struct{ Branches []Branch }
	testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
	testutil.Equal(t, got.Branches[0].Issue, 42)
	testutil.Equal(t, got.Branches[0].IssueURL, "https://github.example/team/repo/issues/42")
}
