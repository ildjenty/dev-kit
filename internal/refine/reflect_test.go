package refine

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestReflectCodexFallbackOrFailure(t *testing.T) {
	for _, tc := range []struct{ name, body, failure string }{
		{"clear", `printf '{"not_reflected":[]}' > "$9"`, ""},
		{"Jev failure", `printf '{"not_reflected":[]}' > "$9"`, ""},
		{"missing decision", `printf '{"not_reflected":[{"decision":"N1","documents":["docs/topic.md"],"reason":"Rejected only"}]}' > "$9"`, ""},
		{"missing executable", "", "unavailable"},
		{"exit", "exit 7", "exit status 7"},
		{"timeout", "exec sleep 5", "timed out"},
		{"malformed", `printf '{"not_reflected":[{}]}' > "$9"`, "does not match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, artifacts := reflectionArtifacts(t, map[string]string{"docs/topic.md": "N1"})
			testutil.Put(t, root, "pack/refine/commands/reflect.md", "Read $ARTIFACTS_DIR/refine/reflect-input.json")
			fake := filepath.Join(root, "fake-codex")
			if tc.body != "" {
				testutil.OK(t, os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'raw Codex output'\n"+tc.body+"\n"), 0700))
			}
			var stdout, stderr bytes.Buffer
			values := map[string]string{"ARTIFACTS_DIR": artifacts, "PACK_DIR": filepath.Join(root, "pack"), "DEVKIT_CODEX": fake}
			if tc.name == "Jev failure" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(402) }))
				defer server.Close()
				values["TYPESAFE_API_KEY"], values["TYPESAFE_API_URL"] = "secret", server.URL
			}
			env := envVars(values)
			env.Cwd, env.Stdout, env.Stderr = root, &stdout, &stderr
			timeout := 5 * time.Second
			if tc.name == "timeout" {
				timeout = 100 * time.Millisecond
			}
			command := cli.Command{Area: "refine", Name: "reflect", Run: func(_ []string, env cli.Env) error { return runReflectTimeout(env, timeout) }}
			code := cli.New("", []cli.Command{command}).Execute([]string{"refine", "reflect"}, env)
			if tc.failure != "" {
				testutil.Equal(t, code, 1)
				testutil.Equal(t, stdout.Len(), 0)
				testutil.Contains(t, stderr.String(), "Jev unavailable: TYPESAFE_API_KEY is not set; Codex unavailable:")
				testutil.Contains(t, stderr.String(), tc.failure)
				return
			}
			testutil.Equal(t, code, 0)
			var result doc
			testutil.OK(t, json.Unmarshal(stdout.Bytes(), &result))
			testutil.Equal(t, result["source"], "codex")
			testutil.Equal(t, len(items(result["not_reflected"])), map[bool]int{true: 1, false: 0}[tc.name == "missing decision"])
			if strings.Contains(stdout.String(), "raw Codex output") {
				t.Fatal("Codex output leaked")
			}
		})
	}
}
