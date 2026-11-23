package judge

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
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRouteCodexFallbackAndConservativeSelection(t *testing.T) {
	for _, tc := range []struct{ name, body, source string }{
		{"success", `printf '{"qualities":[{"quality":"security","file":"odd/file","reason":"Access changed"}]}' > "$9"`, "codex"},
		{"Jev failure", `printf '{"qualities":[]}' > "$9"`, "codex"},
		{"missing", "", "all"},
		{"exit", "exit 3", "all"},
		{"timeout", "exec sleep 5", "all"},
		{"malformed", `printf '{"qualities":[{}]}' > "$9"`, "all"},
		{"unknown quality", `printf '{"qualities":[{"quality":"delivery","file":"odd/file","reason":"Plan only"}]}' > "$9"`, "all"},
		{"unchanged file", `printf '{"qualities":[{"quality":"security","file":"other","reason":"Wrong file"}]}' > "$9"`, "all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := jevInput(t)
			testutil.Put(t, a, "pack/judge/commands/route.md", "Read $ARTIFACTS_DIR/judge/diff.patch and $PACK_DIR.")
			fake := filepath.Join(a, "fake-codex")
			if tc.body != "" {
				testutil.OK(t, os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'raw Codex output'\n"+tc.body+"\n"), 0700))
			}
			var stdout, stderr bytes.Buffer
			values := map[string]string{"ARTIFACTS_DIR": a, "PACK_DIR": filepath.Join(a, "pack"), "DEVKIT_CODEX": fake}
			if tc.name == "Jev failure" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(402) }))
				defer server.Close()
				values["TYPESAFE_API_KEY"], values["TYPESAFE_API_URL"] = "secret", server.URL
			}
			env := cli.Env{Cwd: a, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
			timeout := 5 * time.Second
			if tc.name == "timeout" {
				timeout = 100 * time.Millisecond
			}
			testutil.OK(t, runRouteTimeout(env, timeout))
			var result doc
			testutil.OK(t, json.Unmarshal(stdout.Bytes(), &result))
			testutil.Equal(t, result["source"], tc.source)
			if strings.Contains(stdout.String(), "raw Codex output") {
				t.Fatal("Codex output leaked")
			}
			selected := docs(result["qualities"])
			if tc.source == "codex" {
				if tc.name == "Jev failure" {
					testutil.Equal(t, len(selected), 0)
					return
				}
				testutil.Equal(t, len(selected), 1)
				testutil.Equal(t, selected[0]["quality"], "security")
				return
			}
			mapping := must(concerns.Load(filepath.Join(a, "criteria/fixed")))
			testutil.Equal(t, len(selected), len(mapping.Judged("code")))
			for i, q := range mapping.Judged("code") {
				testutil.Equal(t, selected[i]["quality"], q)
				testutil.Equal(t, selected[i]["file"], "odd/file")
				testutil.Contains(t, text(selected[i]["reason"]), "Jev unavailable:")
				testutil.Contains(t, text(selected[i]["reason"]), "Codex unavailable:")
			}
		})
	}
}
