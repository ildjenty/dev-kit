package claudecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

const testSchema = `{"type":"object","additionalProperties":false,"properties":{"answer":{"type":"string"}},"required":["answer"]}`

func TestRunAddsJudgedTreeReadDirectory(t *testing.T) {
	root, tree := t.TempDir(), t.TempDir()
	testutil.Put(t, root, "pack/check.md", "$ARTIFACTS_DIR $PACK_DIR")
	testutil.Put(t, tree, "peer", "evidence")
	fake := filepath.Join(root, "fake-claude")
	testutil.Put(t, root, "fake-claude", `#!/bin/sh
test "$#" = 13 || exit 1
test "${12}" = --add-dir || exit 2
test -f "${13}/peer" || exit 3
cat > prompt.txt
printf '%s' '{"type":"result","subtype":"success","structured_output":{"answer":"read tree"}}'
`)
	testutil.OK(t, os.Chmod(fake, 0700))
	values := map[string]string{"ARTIFACTS_DIR": "artifacts", "PACK_DIR": "pack", "DEVKIT_CLAUDE": fake}
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: root, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	answer, err := Run(env, "judge", "check.md", testSchema, 10*time.Second, tree)
	testutil.OK(t, err)
	testutil.Equal(t, answer["answer"], "read tree")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, "prompt.txt")))), filepath.Join(root, "artifacts")+" "+filepath.Join(root, "pack"))
}

func TestReadOnlyFlagsStructuredOutputAndOverride(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "PATH", true: "override"}[override], func(t *testing.T) {
			root := t.TempDir()
			testutil.Put(t, root, "pack/check.md", "Read $ARTIFACTS_DIR and $PACK_DIR")
			testutil.Put(t, root, "claude", `#!/bin/sh
set -eu
test "$#" = 11
test "$1 $2" = '-p --json-schema'
test "$4 $5 $6 $7 $8 $9 ${10} ${11}" = '--allowedTools Read,Grep,Glob --tools Read,Grep,Glob --permission-mode dontAsk --output-format json'
printf '%s' "$3" > schema.txt
cat > prompt.txt
printf 'secret diagnostics' >&2
printf '%s' '{"type":"result","subtype":"success","is_error":false,"result":"plain answer is ignored","structured_output":{"answer":"secret accepted"}}'
`)
			testutil.OK(t, os.Chmod(filepath.Join(root, "claude"), 0700))
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			values := map[string]string{"PACK_DIR": filepath.Join(root, "pack"), "ARTIFACTS_DIR": filepath.Join(root, "artifacts"), "TYPESAFE_API_KEY": "secret"}
			if override {
				values["DEVKIT_CLAUDE"] = filepath.Join(root, "claude")
			}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: root, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
			result, err := Run(env, "judge", "check.md", testSchema, 10*time.Second)
			testutil.OK(t, err)
			testutil.Equal(t, result["answer"], "[redacted] accepted")
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, "schema.txt")))), testSchema)
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, "prompt.txt")))), "Read "+values["ARTIFACTS_DIR"]+" and "+values["PACK_DIR"])
			testutil.Equal(t, out.Len(), 0)
			testutil.Equal(t, strings.Contains(stderr.String(), "secret"), false)
			logs := testutil.Must(filepath.Glob(filepath.Join(root, "artifacts/judge/claude-*.log")))
			testutil.Equal(t, len(logs), 1)
			log := string(testutil.Must(os.ReadFile(logs[0])))
			testutil.Equal(t, strings.Contains(log, "secret"), false)
			testutil.Contains(t, log, "[redacted] diagnostics")
		})
	}
}

func TestClaudeFailuresAndSecretRedaction(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"missing", "", "unavailable"},
		{"exit", "printf 'secret' >&2; exit 7", "exit status 7"},
		{"timeout", "exec sleep 5", "timed out"},
		{"invalid JSON", `printf broken`, "does not match"},
		{"text result", `printf '%s' '{"type":"result","subtype":"success","result":"{\"answer\":\"ok\"}"}'`, "does not match"},
		{"error flag", `printf '%s' '{"type":"result","subtype":"success","is_error":true,"structured_output":{"answer":"ok"}}'`, "does not match"},
		{"error subtype", `printf '%s' '{"type":"result","subtype":"error_max_turns","structured_output":{"answer":"ok"}}'`, "does not match"},
		{"schema", `printf '%s' '{"type":"result","subtype":"success","structured_output":{"answer":3}}'`, "does not match"},
		{"extra", `printf '%s' '{"type":"result","subtype":"success","structured_output":{"answer":"ok","extra":false}}'`, "does not match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testutil.Put(t, root, "pack/check.md", "prompt")
			fake := filepath.Join(root, "fake-claude")
			if tc.body != "" {
				testutil.OK(t, os.WriteFile(fake, []byte("#!/bin/sh\n"+tc.body+"\n"), 0700))
			}
			var out, stderr bytes.Buffer
			values := map[string]string{"PACK_DIR": filepath.Join(root, "pack"), "ARTIFACTS_DIR": root, "DEVKIT_CLAUDE": fake, "TYPESAFE_API_KEY": "secret"}
			env := cli.Env{Cwd: root, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
			limit := 10 * time.Second
			if tc.name == "timeout" {
				limit = 50 * time.Millisecond
			}
			started := time.Now()
			_, err := Run(env, "refine", "check.md", testSchema, limit)
			testutil.Fails(t, err, tc.want)
			if tc.name == "timeout" && time.Since(started) > time.Second {
				t.Fatal("timeout not enforced")
			}
			testutil.Equal(t, out.Len(), 0)
			testutil.Equal(t, strings.Contains(stderr.String(), "secret"), false)
		})
	}
}
