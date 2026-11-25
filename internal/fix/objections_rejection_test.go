package fix

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

func TestObjectionsCLIPreservesUnreadableInputAndState(t *testing.T) {
	for _, input := range []string{"malformed", "read-error", "null", "{}", `{"objections":"broken"}`, `{"objections":null}`, `{"objections":[null]}`, `{"objections":[{}]}`, `{"objections":[{"finding":1,"evidence":"proof"}]}`, `{"objections":[{"finding":"F1","evidence":1}]}`, "missing", "empty", "objection"} {
		t.Run(input, func(t *testing.T) {
			ctx, artifacts, _, head := setup(t, 3)
			testutil.Must(prepare(ctx, artifacts))
			source := filepath.Join(artifacts, "objections.json")
			switch input {
			case "malformed":
				testutil.OK(t, os.WriteFile(source, []byte(`{"objections":[`), 0644))
			case "read-error":
				testutil.OK(t, os.Mkdir(source, 0755))
			case "empty":
				putJSON(t, source, record.Document{"objections": []record.Document{}})
			case "objection":
				putJSON(t, source, record.Document{"objections": []record.Document{{"finding": "j1/contract/F1", "evidence": "The guard rejects it"}}})
			default:
				if input != "missing" {
					testutil.OK(t, os.WriteFile(source, []byte(input), 0644))
				}
			}
			file := filepath.Join(artifacts, "fix/state.json")
			before := testutil.Must(os.ReadFile(file))
			findings := testutil.Must(os.ReadFile(filepath.Join(artifacts, "fix/findings.json")))
			var stdout, stderr bytes.Buffer
			vars := map[string]string{"ARTIFACTS_DIR": artifacts}
			env := cli.Env{Cwd: ctx.Cwd, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
			code := cli.New("test", Commands()).Execute([]string{"fix", "objections"}, env)
			if input != "missing" && input != "empty" && input != "objection" {
				testutil.Equal(t, code, 1)
				testutil.Equal(t, stdout.String(), "")
				if stderr.Len() == 0 {
					t.Fatal("objection read failure was not reported")
				}
				testutil.Equal(t, string(testutil.Must(os.ReadFile(file))), string(before))
				if input == "read-error" {
					testutil.Equal(t, testutil.Must(os.Stat(source)).IsDir(), true)
				} else {
					testutil.Must(os.Stat(source))
				}
				testutil.Contains(t, stderr.String(), source)
				_, err := os.Stat(filepath.Join(artifacts, "fix/objections"))
				if !os.IsNotExist(err) {
					t.Fatalf("rejected objections were consumed: %v", err)
				}
			} else {
				testutil.Equal(t, code, 0)
				var got record.Document
				testutil.OK(t, json.Unmarshal(stdout.Bytes(), &got))
				count := 0
				if input == "objection" {
					count = 1
				}
				testutil.Equal(t, got, record.Document{"round": 1, "count": count})
				s := testutil.Must(readJSON[state](file))
				testutil.Equal(t, len(s.Current), count)
				testutil.Equal(t, s.Round, 1)
				if input != "missing" {
					_, err := os.Stat(source)
					if !os.IsNotExist(err) {
						t.Fatalf("successful input was not consumed: %v", err)
					}
					testutil.Must(os.Stat(filepath.Join(artifacts, "fix/objections/1.written.json")))
				}
			}
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "fix/findings.json")))), string(findings))
			testutil.Equal(t, len(testutil.Must(record.List(ctx.Dir, "result"))), 0)
			testutil.Equal(t, len(testutil.Must(record.List(ctx.Dir, "judgment"))), 1)
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
		})
	}
}
