package status

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

func TestStatusReportsConfiguredRestartLimitAndUnreadableConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, config, cause string
		limit               int
	}{
		{name: "missing", limit: 3},
		{name: "defaults", config: `{}`, limit: 3},
		{name: "zero", config: `{"limits":{"restarts":0}}`, cause: "limits.restarts must be an integer of at least 1", limit: -1},
		{name: "positive", config: `{"limits":{"restarts":2}}`, limit: 2},
		{name: "negative", config: `{"limits":{"restarts":-1}}`, cause: "limits.restarts must be an integer of at least 1", limit: -1},
		{name: "wrong type", config: `{"limits":{"restarts":"0"}}`, cause: "limits.restarts must be an integer of at least 1", limit: -1},
		{name: "malformed", config: `{`, cause: "unexpected end of JSON input", limit: -1},
		{name: "read failure", cause: "is a directory", limit: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, state := testutil.Repo(t), t.TempDir()
			if tc.config != "" {
				testutil.Put(t, repo, ".devkit/config.json", tc.config)
			} else if tc.name == "read failure" {
				testutil.OK(t, os.MkdirAll(filepath.Join(repo, ".devkit/config.json"), 0755))
			}
			for _, name := range []string{"feature/x", "feature/running", "feature/pass"} {
				dir := branch(t, filepath.Join(state, "records"), name)
				var result map[string]any
				switch name {
				case "feature/x":
					result = map[string]any{"outcome": "stop", "stop": map[string]any{"state": "Units remain", "unresolved": []any{"U1"}}}
				case "feature/pass":
					result = map[string]any{"outcome": "pass"}
				}
				run1(t, dir, "implement", 1, result)
			}
			beforeState, beforeRepo := statusFiles(t, state), statusFiles(t, repo)
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { return state, k == "STATE_DIR" }}
			registry := cli.New("", Commands())
			for _, filter := range []string{"", "feature/x"} {
				args := []string{"status", "--json"}
				if filter != "" {
					args = append(args, filter)
				}
				out.Reset()
				testutil.Equal(t, registry.Execute(args, env), 0)
				var parsed struct{ Branches []Branch }
				testutil.OK(t, json.Unmarshal(out.Bytes(), &parsed))
				count := 3
				if filter != "" {
					count = 1
				}
				testutil.Equal(t, len(parsed.Branches), count)
				for _, b := range parsed.Branches {
					testutil.Equal(t, b.Limit, tc.limit)
					testutil.Equal(t, b.Restarts, 0)
					testutil.Equal(t, b.Workflow, "implement")
					testutil.Equal(t, b.RunID, "implement-1")
					if tc.cause != "" {
						testutil.Equal(t, len(b.Errors), 1)
						testutil.Contains(t, b.Errors[0], ".devkit/config.json")
						testutil.Contains(t, b.Errors[0], tc.cause)
						testutil.Contains(t, b.Errors[0], "restart limit is unknown")
						testutil.Equal(t, b.Next, "Resolve the unreadable configuration first, then check status again.")
						continue
					}
					testutil.Equal(t, b.Errors, []string{})
					switch b.Branch {
					case "feature/x":
						testutil.Equal(t, b.Outcome, "stop")
						testutil.Equal(t, strings.Contains(b.Next, "--input continue=..."), false)
					case "feature/running":
						testutil.Equal(t, b.Outcome, "no result")
						testutil.Contains(t, b.Next, "archon workflow get implement-1")
					case "feature/pass":
						testutil.Equal(t, b.Outcome, "pass")
						testutil.Equal(t, b.Next, "Start judge on feature/pass.")
					}
				}
				out.Reset()
				args = []string{"status"}
				if filter != "" {
					args = append(args, filter)
				}
				testutil.Equal(t, registry.Execute(args, env), 0)
				for _, b := range parsed.Branches {
					testutil.Contains(t, out.String(), b.Branch+": implement "+b.Outcome)
					testutil.Contains(t, out.String(), "  next: "+b.Next)
					for _, err := range b.Errors {
						testutil.Contains(t, out.String(), "  unreadable: "+err)
					}
				}
				testutil.Equal(t, strings.Count(out.String(), "  unreadable: "), count*len(parsed.Branches[0].Errors))
			}
			testutil.Equal(t, stderr.String(), "")
			testutil.Equal(t, statusFiles(t, state), beforeState)
			testutil.Equal(t, statusFiles(t, repo), beforeRepo)
		})
	}
}
