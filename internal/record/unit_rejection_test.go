package record

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRequirementsStartCLIRejectsInvalidUnitListsBeforeWriting(t *testing.T) {
	for _, source := range []string{"input", "request"} {
		for _, tc := range []struct{ name, fields, reason string }{
			{"scalar-dependency", "depends_on: U-bbbbbb\ndecisions: []", "depends_on must be a list"},
			{"missing-dependency", "decisions: []", "depends_on must be a list"},
			{"empty-scalar-dependency", "depends_on:\ndecisions: []", "depends_on must be a list"},
			{"scalar-decision", "depends_on: []\ndecisions: D-aaaaaa", "decisions must be a list"},
			{"missing-decision", "depends_on: []", "decisions must be a list"},
			{"empty-scalar-decision", "depends_on: []\ndecisions:", "decisions must be a list"},
			{"waiting", "depends_on: [U-bbbbbb]\ndecisions: []", "unit U-aaaaaa waits for U-bbbbbb"},
			{"proposed", "depends_on: []\ndecisions: [D-aaaaaa]", "decision D-aaaaaa which is not agreed"},
			{"empty-lists", "depends_on: []\ndecisions: []", ""},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				repo, state := testutil.Repo(t), t.TempDir()
				testutil.Put(t, repo, ".devkit/units/U-aaaaaa.md", "---\nid: U-aaaaaa\ntitle: Unit\n"+tc.fields+"\n---\nGoal\n")
				testutil.Put(t, repo, ".devkit/units/U-bbbbbb.md", "---\nid: U-bbbbbb\ntitle: Dependency\ndecisions: []\ndepends_on: []\n---\nNot done\n")
				testutil.Put(t, repo, ".devkit/decisions/D-aaaaaa.md", "---\nid: D-aaaaaa\nstatus: proposed\n---\nNot agreed\n")
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "unit inputs")
				dir := RecordDir(state, "feature/x")
				input := "U-aaaaaa"
				if source == "request" {
					testutil.Put(t, dir, "request/1.json", `{"unit":"U-aaaaaa"}`)
					input = ""
				}
				var out, stderr bytes.Buffer
				env := commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1", "INPUTS_UNIT": input}, &out, &stderr)
				testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"record", "start", "requirements"}, env), 0)
				var got StartOutput
				testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
				testutil.Equal(t, got.Stopped, tc.reason != "")
				if tc.reason != "" {
					testutil.Contains(t, strings.Join(got.Remaining, "\n"), tc.reason)
					for _, kind := range []string{"run", "result", "request", "continuation"} {
						want := 0
						if kind == "request" && source == "request" {
							want = 1
						}
						testutil.Equal(t, len(must(List(dir, kind))), want)
					}
					testutil.Equal(t, len(must(ListKeyed(dir, "answer"))), 0)
					if source == "request" {
						testutil.Equal(t, string(must(os.ReadFile(filepath.Join(dir, "request/1.json")))), `{"unit":"U-aaaaaa"}`)
					}
				}
				testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
			})
		}
	}
}
