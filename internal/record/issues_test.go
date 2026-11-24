package record

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRequirementsIssueStartConditions(t *testing.T) {
	for _, source := range []string{"input", "request"} {
		for _, tc := range []struct{ name, state, label, blocker, reason string }{
			{"missing", "missing", "work", "", "issue #42 could not be read"},
			{"closed", "CLOSED", "work", "", "issue #42 is not open"},
			{"wrong label", "OPEN", "devkit:unit", "", "issue #42 does not carry unit label"},
			{"blocked", "OPEN", "work", "open", "issue #42 is blocked by issue #8"},
			{"ready", "OPEN", "work", "closed", ""},
			{"no blockers", "OPEN", "work", "", ""},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				repo, state := testutil.Repo(t), t.TempDir()
				gh := testutil.IssueCLI(t, tc.state, tc.label, tc.blocker)
				testutil.Put(t, repo, ".devkit/config.json", `{"issues":{"labels":{"unit":"work"}}}`)
				testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "malformed decision record")
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "issue configuration")
				dir := RecordDir(state, "feature/x")
				input := "42"
				if source == "request" {
					testutil.Put(t, dir, "request/1.json", `{"issue":42}`)
					input = ""
				}
				var out, stderr bytes.Buffer
				env := commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1", "INPUTS_ISSUE": input, "DEVKIT_GH": gh}, &out, &stderr)
				env.Environ = os.Environ()
				testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"record", "start", "requirements"}, env), 0)
				var got StartOutput
				testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
				testutil.Equal(t, got.Stopped, tc.reason != "")
				if tc.reason != "" {
					testutil.Contains(t, strings.Join(got.Remaining, "\n"), tc.reason)
					testutil.Equal(t, len(must(List(dir, "run"))), 0)
					testutil.Equal(t, len(must(List(dir, "result"))), 0)
				}
				testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
			})
		}
	}
}

func TestIssueInputRequiresPositiveNumber(t *testing.T) {
	for _, value := range []string{"0", "-1", "U-abcdef", "42.5"} {
		env := cli.Env{LookupEnv: func(string) (string, bool) { return value, true }}
		_, err := InputIssue(env, t.TempDir())
		testutil.Fails(t, err, "positive issue number")
	}
}
