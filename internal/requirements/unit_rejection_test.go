package requirements

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPrepareCLIRejectsInvalidUnitListsBeforeWriting(t *testing.T) {
	for _, source := range []string{"input", "request"} {
		for _, tc := range []struct{ name, fields, reason string }{
			{"scalar-dependency", "depends_on: U-bbbbbb\ndecisions: []", "depends_on must be a list"},
			{"missing-dependency", "decisions: []", "depends_on must be a list"},
			{"empty-scalar-dependency", "depends_on:\ndecisions: []", "depends_on must be a list"},
			{"scalar-decision", "depends_on: []\ndecisions: D-aaaaaa", "decisions must be a list"},
			{"missing-decision", "depends_on: []", "decisions must be a list"},
			{"empty-scalar-decision", "depends_on: []\ndecisions:", "decisions must be a list"},
			{"proposed", "depends_on: []\ndecisions: [D-aaaaaa]", "decision D-aaaaaa which is not agreed"},
			{"empty-lists", "depends_on: []\ndecisions: []", ""},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				repo, state, artifacts := testutil.Repo(t), t.TempDir(), t.TempDir()
				testutil.Standards(t, repo, ".devkit/standards")
				testutil.Put(t, repo, ".devkit/units/U-aaaaaa.md", "---\nid: U-aaaaaa\ntitle: Unit\n"+tc.fields+"\n---\nGoal\n")
				testutil.Put(t, repo, ".devkit/units/U-bbbbbb.md", "---\nid: U-bbbbbb\ntitle: Dependency\ndecisions: []\ndepends_on: []\n---\nNot done\n")
				testutil.Put(t, repo, ".devkit/decisions/D-aaaaaa.md", "---\nid: D-aaaaaa\nstatus: proposed\n---\nNot agreed\n")
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "unit inputs")
				testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
				dir := record.RecordDir(state, "feature/x")
				run(dir, "r1")
				input := "U-aaaaaa"
				if source == "request" {
					must(record.Write(record.ContextData{Cwd: repo, BaseBranch: "main", Dir: dir, RunID: "r1"}, "request", document{"owner_words": "", "supplement": "", "unit": input}, ""))
					input = ""
				}
				requests := must(record.List(dir, "request"))
				testutil.Put(t, artifacts, "requirements/input.json", "previous input\n")
				var out, stderr bytes.Buffer
				vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": artifacts, "INPUTS_UNIT": input, "INPUTS_OWNER_WORDS": "An additional request"}
				env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
				code := cli.New("test", Commands()).Execute([]string{"requirements", "prepare"}, env)
				if tc.reason != "" {
					testutil.Equal(t, code, 1)
					testutil.Contains(t, stderr.String(), tc.reason)
					testutil.Equal(t, out.String(), "")
					testutil.Equal(t, must(record.List(dir, "request")), requests)
					testutil.Equal(t, string(must(os.ReadFile(filepath.Join(artifacts, "requirements/input.json")))), "previous input\n")
					_, err := os.Stat(filepath.Join(artifacts, "criteria"))
					if !os.IsNotExist(err) {
						t.Fatalf("criteria were copied on rejection: %v", err)
					}
				} else {
					testutil.Equal(t, code, 0)
					var prepared document
					testutil.OK(t, json.Unmarshal(out.Bytes(), &prepared))
					got := must(read(text(prepared["input"])))
					testutil.Equal(t, got["agreed_decisions"], []document{})
					testutil.Equal(t, object(got["unit"])["depends_on"], []string{})
				}
				testutil.Equal(t, len(must(record.List(dir, "run"))), 1)
				testutil.Equal(t, len(must(record.List(dir, "result"))), 0)
				testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
			})
		}
	}
}
