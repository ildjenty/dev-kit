package codexcli

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

func TestRunUsesJudgedTreeWithoutGitMetadata(t *testing.T) {
	root, tree := t.TempDir(), t.TempDir()
	testutil.Put(t, root, "pack/check.md", "$ARTIFACTS_DIR $PACK_DIR")
	testutil.Put(t, tree, "peer", "evidence")
	fake := filepath.Join(root, "fake-codex")
	testutil.Put(t, root, "fake-codex", `#!/bin/sh
test "$5" = "$PWD" || exit 1
test -f peer || exit 2
test ! -e .git || exit 3
test "$1 $2 $3 $4" = 'exec --sandbox read-only --cd' || exit 4
test "${10} ${11}" = '--skip-git-repo-check -' || exit 5
case "$(cat)" in /*' '/*) ;; *) exit 6 ;; esac
printf '{"answer":"read tree"}' > "$9"
`)
	testutil.OK(t, os.Chmod(fake, 0700))
	values := map[string]string{"ARTIFACTS_DIR": "artifacts", "PACK_DIR": "pack", "DEVKIT_CODEX": fake}
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: root, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	answer, err := Run(env, "judge", "check.md", testSchema, 10*time.Second, tree)
	testutil.OK(t, err)
	testutil.Equal(t, answer["answer"], "read tree")
}

func TestRunReadOnlySchemaPromptAndLogs(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "PATH", true: "override"}[override], func(t *testing.T) {
			root := t.TempDir()
			testutil.Put(t, root, "pack/check.md", "$ARTIFACTS_DIR and $PACK_DIR\n")
			testutil.Put(t, root, "codex", `#!/bin/sh
test "$#" = 11 || exit 11
test "$1 $2 $3 $4" = 'exec --sandbox read-only --cd' || exit 12
test "$5" = "$PWD" || exit 13
test "$6" = --output-schema && test "$8" = -o && test "${10}" = --skip-git-repo-check && test "${11}" = - || exit 14
test -s "$7" || exit 15
cat > prompt.txt
printf '%s' "$7" > schema-path.txt
printf 'Codex stdout secret\n'
printf 'Codex stderr secret\n' >&2
printf '{"answer":"secret accepted"}' > "$9"
`)
			testutil.OK(t, os.Chmod(filepath.Join(root, "codex"), 0700))
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			values := map[string]string{"ARTIFACTS_DIR": filepath.Join(root, "artifacts"), "PACK_DIR": filepath.Join(root, "pack"), "TYPESAFE_API_KEY": "secret"}
			if override {
				values["DEVKIT_CODEX"] = filepath.Join(root, "codex")
			}
			var stdout, stderr bytes.Buffer
			env := cli.Env{Cwd: root, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
			answer, err := Run(env, "judge", "check.md", testSchema, 10*time.Second)
			testutil.OK(t, err)
			testutil.Equal(t, answer["answer"], "[redacted] accepted")
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, "prompt.txt")))), values["ARTIFACTS_DIR"]+" and "+values["PACK_DIR"]+"\n")
			testutil.Equal(t, stdout.Len(), 0)
			if strings.Contains(stderr.String(), "secret") || strings.Contains(stderr.String(), "Codex stdout") {
				t.Fatal("output leaked")
			}
			schemaPath := string(testutil.Must(os.ReadFile(filepath.Join(root, "schema-path.txt"))))
			_, err = os.Stat(filepath.Dir(schemaPath))
			testutil.Equal(t, os.IsNotExist(err), true)
			logs := testutil.Must(filepath.Glob(filepath.Join(root, "artifacts/judge/codex-*.log")))
			testutil.Equal(t, len(logs), 1)
			testutil.Contains(t, string(testutil.Must(os.ReadFile(logs[0]))), "Codex stdout secret\nCodex stderr secret")
		})
	}
}

func TestRunFailuresAndBoundedRedactedTail(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"missing", "", "unavailable"},
		{"exit", "printf 'secret\\n'\nexit 9", "exit status 9"},
		{"timeout", "exec sleep 5", "timed out"},
		{"invalid JSON", "printf 'broken' > \"$9\"", "does not match"},
		{"invalid schema", `printf '{"answer":1}' > "$9"`, "does not match"},
		{"missing answer", "exit 0", "does not match"},
		{"extra property", `printf '{"answer":"ok","extra":true}' > "$9"`, "does not match"},
		{"tail", "printf 'HEAD'\ni=0; while [ $i -lt 5000 ]; do printf '界'; i=$((i+1)); done\nprintf 'secretTAIL'; exit 1", "exit status 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testutil.Put(t, root, "pack/check.md", "prompt")
			executable := filepath.Join(root, "fake-codex")
			if tc.body != "" {
				testutil.OK(t, os.WriteFile(executable, []byte("#!/bin/sh\n"+tc.body+"\n"), 0700))
			}
			var stdout, stderr bytes.Buffer
			values := map[string]string{"ARTIFACTS_DIR": root, "PACK_DIR": filepath.Join(root, "pack"), "DEVKIT_CODEX": executable, "TYPESAFE_API_KEY": "secret"}
			env := cli.Env{Cwd: root, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
			timeout := 10 * time.Second
			if tc.name == "timeout" {
				timeout = 100 * time.Millisecond
			}
			started := time.Now()
			_, err := Run(env, "refine", "check.md", testSchema, timeout)
			testutil.Fails(t, err, tc.want)
			if tc.name == "timeout" && time.Since(started) > 2*time.Second {
				t.Fatal("timeout was not enforced")
			}
			testutil.Equal(t, stdout.Len(), 0)
			if strings.Contains(stderr.String(), "secret") || strings.Contains(stderr.String(), "HEAD") {
				t.Fatal("secret or discarded log prefix leaked")
			}
			if tc.name == "tail" {
				lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
				testutil.Equal(t, len([]rune(lines[len(lines)-1])), 4000)
				testutil.Equal(t, strings.HasSuffix(stderr.String(), "[redacted]TAIL\n"), true)
			}
		})
	}
}
